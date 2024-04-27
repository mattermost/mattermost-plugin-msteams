package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	goPlugin "github.com/hashicorp/go-plugin"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type testHelper struct {
	p                *Plugin
	appClientMock    *mocks.Client
	clientMock       *mocks.Client
	websocketClients map[string]*model.WebSocketClient
}

func setupTestHelper(t *testing.T) *testHelper {
	setupReattachEnvironment(t)

	t.Helper()

	p := &Plugin{
		// These mocks are replaced later, but serve the plugin during early initialization
		msteamsAppClient: &mocks.Client{},
		clientBuilderWithToken: func(redirectURL, tenantID, clientId, clientSecret string, token *oauth2.Token, apiClient *pluginapi.LogService) msteams.Client {
			return &mocks.Client{}
		},
	}
	th := &testHelper{
		p: p,
	}

	// ctx, and specifically cancel, gives us control over the plugin lifecycle
	ctx, cancel := context.WithCancel(context.Background())

	// reattachConfigCh is the means by which we get the Unix socket information to relay back
	// to the server and finish the reattachment.
	reattachConfigCh := make(chan *goPlugin.ReattachConfig)

	// closeCh tells us when the plugin exits and allows for cleanup.
	closeCh := make(chan struct{})

	// plugin.ClientMain with options allows for reattachment.
	go plugin.ClientMain(
		th.p,
		plugin.WithTestContext(ctx),
		plugin.WithTestReattachConfigCh(reattachConfigCh),
		plugin.WithTestCloseCh(closeCh),
	)

	// Make sure the plugin shuts down normally with the test
	t.Cleanup(func() {
		cancel()

		select {
		case <-closeCh:
		case <-time.After(5 * time.Second):
			panic("plugin failed to close after 5 seconds")
		}
	})

	// Wait for the plugin to start and then reattach to the server.
	var reattachConfig *goPlugin.ReattachConfig
	select {
	case reattachConfig = <-reattachConfigCh:
	case <-time.After(5 * time.Second):
		t.Fatal("failed to get reattach config")
	}

	// Reattaching requires a local mode client.
	socketPath := os.Getenv("MM_LOCALSOCKETPATH")
	if socketPath == "" {
		socketPath = model.LocalModeSocketPath
	}
	clientLocal := model.NewAPIv4SocketClient(socketPath)

	// Set the plugin config before reattaching. This is unique to MS Teams because the plugin
	// currently fails to start without a valid configuration.
	_, _, err := clientLocal.PatchConfig(ctx, &model.Config{
		PluginSettings: model.PluginSettings{
			Plugins: map[string]map[string]any{
				manifest.Id: {
					"tenantid":                model.NewId(),
					"clientid":                model.NewId(),
					"clientsecret":            model.NewId(),
					"encryptionkey":           "aaaaaaaaaaaaaaaaaaaaaaaaaaaa_aaa",
					"webhooksecret":           model.NewId(),
					"syncusers":               0,
					"disableCheckCredentials": true,
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = clientLocal.ReattachPlugin(ctx, &model.PluginReattachRequest{
		Manifest:             manifest,
		PluginReattachConfig: model.NewPluginReattachConfig(reattachConfig),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, err := clientLocal.DetachPlugin(ctx, manifest.Id)
		require.NoError(t, err)
	})

	th.Reset(t)
	return th
}

func (th *testHelper) clearDatabase(t *testing.T) {
	db, err := th.p.apiClient.Store.GetMasterDB()
	require.NoError(t, err)

	_, err = db.Exec("DELETE FROM msteamssync_links")
	require.NoError(t, err)
	_, err = db.Exec("DELETE FROM msteamssync_invited_users")
	require.NoError(t, err)
	_, err = db.Exec("DELETE FROM msteamssync_posts")
	require.NoError(t, err)
	_, err = db.Exec("DELETE FROM msteamssync_subscriptions")
	require.NoError(t, err)
	_, err = db.Exec("DELETE FROM msteamssync_users")
	require.NoError(t, err)
	_, err = db.Exec("DELETE FROM msteamssync_whitelist")
	require.NoError(t, err)
}

func (th *testHelper) Reset(t *testing.T) *testHelper {
	t.Helper()

	// Wipe the tables for this plugin to ensure a clean slate. Note that we don't currently
	// touch any Mattermost tables.
	th.clearDatabase(t)

	appClientMock := &mocks.Client{}
	clientMock := &mocks.Client{}
	appClientMock.Test(t)
	clientMock.Test(t)

	th.appClientMock = appClientMock
	th.clientMock = clientMock

	th.p.msteamsAppClient = appClientMock
	th.p.clientBuilderWithToken = func(redirectURL, tenantID, clientId, clientSecret string, token *oauth2.Token, apiClient *pluginapi.LogService) msteams.Client {
		return clientMock
	}

	t.Cleanup(func() {
		appClientMock.AssertExpectations(t)
		clientMock.AssertExpectations(t)

		// Ccheck the websocket event queue for unhandled events that might represent
		// unexpected behavior.
		unmatchedEvents := make(map[string][]*model.WebSocketEvent)

	nextWebsocketClient:
		for userID, websocketClient := range th.websocketClients {
			for {
				select {
				case event := <-websocketClient.EventChannel:
					switch event.EventType() {
					case model.WebsocketEventHello, model.WebsocketEventStatusChange:
						// Ignore these common events by default.
						continue
					default:
						unmatchedEvents[userID] = append(unmatchedEvents[userID], event)
					}
				default:
					continue nextWebsocketClient
				}
			}
		}

		for userID, events := range unmatchedEvents {
			t.Logf("found %d unmatched websocket events for user %s", len(events), userID)
			for _, event := range events {
				t.Logf(" - %s", event.EventType())
			}
		}
		if len(unmatchedEvents) > 0 {
			t.Fail()
		}
	})

	return th
}

func (th *testHelper) SetupTeam(t *testing.T) *model.Team {
	t.Helper()

	teamName := model.NewRandomTeamName()
	team, appErr := th.p.API.CreateTeam(&model.Team{
		Name:        teamName,
		DisplayName: teamName,
		Type:        model.TeamOpen,
	})
	require.Nil(t, appErr)

	return team
}

type ChannelOption func(*testing.T, *testHelper, *model.Channel)

func WithMembers(members ...*model.User) ChannelOption {
	return func(t *testing.T, th *testHelper, channel *model.Channel) {
		t.Helper()

		for _, user := range members {
			_, appErr := th.p.API.AddUserToChannel(channel.Id, user.Id, user.Id)
			require.Nil(t, appErr)
		}
	}
}

func (th *testHelper) SetupPublicChannel(t *testing.T, team *model.Team, opts ...ChannelOption) *model.Channel {
	t.Helper()

	channelName := model.NewId()
	channel, appErr := th.p.API.CreateChannel(&model.Channel{
		Name:        channelName,
		DisplayName: channelName,
		Type:        model.ChannelTypeOpen,
		TeamId:      team.Id,
	})
	require.Nil(t, appErr)

	for _, opt := range opts {
		opt(t, th, channel)
	}

	return channel
}

func (th *testHelper) SetupPrivateChannel(t *testing.T, team *model.Team, opts ...ChannelOption) *model.Channel {
	t.Helper()

	channelName := model.NewId()
	channel, appErr := th.p.API.CreateChannel(&model.Channel{
		Name:        channelName,
		DisplayName: channelName,
		Type:        model.ChannelTypePrivate,
		TeamId:      team.Id,
	})
	require.Nil(t, appErr)

	for _, opt := range opts {
		opt(t, th, channel)
	}

	return channel
}

func (th *testHelper) LinkChannel(t *testing.T, team *model.Team, channel *model.Channel, user *model.User) {
	channelLink := storemodels.ChannelLink{
		MattermostTeamID:    team.Id,
		MattermostChannelID: channel.Id,
		MSTeamsTeam:         model.NewId(),
		MSTeamsChannel:      model.NewId(),
		Creator:             user.Id,
	}
	err := th.p.store.StoreChannelLink(&channelLink)
	require.NoError(t, err)
}

func (th *testHelper) SetupUser(t *testing.T, team *model.Team) *model.User {
	t.Helper()

	username := model.NewId()

	user := &model.User{
		Email:         fmt.Sprintf("%s@example.com", username),
		Username:      username,
		Password:      "password",
		EmailVerified: true,
	}

	user, appErr := th.p.API.CreateUser(user)
	require.Nil(t, appErr)

	_, appErr = th.p.API.CreateTeamMember(team.Id, user.Id)
	require.Nil(t, appErr)

	return user
}

func (th *testHelper) SetupRemoteUser(t *testing.T, team *model.Team) *model.User {
	t.Helper()

	username := fmt.Sprintf("msteams_%s", model.NewId())
	remoteID := &th.p.remoteID

	user := &model.User{
		Email:         fmt.Sprintf("%s@example.com", username),
		Username:      username,
		Password:      "password",
		EmailVerified: true,
		RemoteId:      remoteID,
	}

	user, appErr := th.p.API.CreateUser(user)
	require.Nil(t, appErr)

	_, appErr = th.p.API.CreateTeamMember(team.Id, user.Id)
	require.Nil(t, appErr)

	return user
}

func (th *testHelper) SetupSyntheticUser(t *testing.T, team *model.Team) *model.User {
	t.Helper()

	user := th.SetupRemoteUser(t, team)
	teamsID := "t" + user.Id
	err := th.p.store.SetUserInfo(user.Id, teamsID, nil)
	require.NoError(t, err)

	return user
}

func (th *testHelper) ConnectUser(t *testing.T, userID string) {
	teamID := "t" + userID
	err := th.p.store.SetUserInfo(userID, teamID, &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
	require.NoError(t, err)
}

func (th *testHelper) SetupConnectedUser(t *testing.T, team *model.Team) *model.User {
	t.Helper()

	user := th.SetupUser(t, team)
	th.ConnectUser(t, user.Id)

	return user
}

func (th *testHelper) SetupSysadmin(t *testing.T, team *model.Team) *model.User {
	t.Helper()

	var user *model.User
	var err error
	user = th.SetupUser(t, team)

	user, err = th.p.apiClient.User.UpdateRoles(user.Id, model.SystemUserRoleId+" "+model.SystemAdminRoleId)
	require.NoError(t, err)

	return user
}

func (th *testHelper) SetupClient(t *testing.T, userID string) *model.Client4 {
	t.Helper()

	user, err := th.p.apiClient.User.Get(userID)
	require.NoError(t, err)

	client := model.NewAPIv4Client(os.Getenv("MM_SERVICESETTINGS_SITEURL"))

	// TODO: Don't hardcode "password"
	_, _, err = client.Login(context.TODO(), user.Username, "password")
	require.NoError(t, err)

	return client
}

func (th *testHelper) setupWebsocketClient(t *testing.T, client *model.Client4) *model.WebSocketClient {
	t.Helper()

	websocketURL, err := url.Parse(client.URL)
	require.NoError(t, err)

	if websocketURL.Scheme == "http" {
		websocketURL.Scheme = "ws"
	} else if websocketURL.Scheme == "https" {
		websocketURL.Scheme = "wss"
	} else {
		t.Fatalf("unexpected client scheme: %s", websocketURL.Scheme)
	}

	websocketClient, err := model.NewWebSocketClient(websocketURL.String(), client.AuthToken)
	require.NoError(t, err)

	return websocketClient
}

// SetupWebsocketClientForUser sets up a websocket client for a user.
//
// Call this before you emit the event in order to ensure the client is ready to receive it.
func (th *testHelper) SetupWebsocketClientForUser(t *testing.T, userID string) {
	t.Helper()

	if th.websocketClients == nil {
		th.websocketClients = make(map[string]*model.WebSocketClient)
	}

	if websocketClient := th.websocketClients[userID]; websocketClient == nil {
		client := th.SetupClient(t, userID)
		websocketClient = th.setupWebsocketClient(t, client)
		websocketClient.Listen()
		t.Cleanup(func() {
			websocketClient.Close()
			delete(th.websocketClients, userID)
		})

		th.websocketClients[userID] = websocketClient
	}
}

// GetWebsocketClientForUser returns a websocket client previously setup for a user.
//
// It's important to call SetupWebsocketClientForUser first and early in tests, otherwise the
// websocket won't be setup to listen to the event of interest. To help with this, this method
// won't create a websocket client on demand.
func (th *testHelper) GetWebsocketClientForUser(t *testing.T, userID string) *model.WebSocketClient {
	t.Helper()

	if th.websocketClients == nil {
		th.websocketClients = make(map[string]*model.WebSocketClient)
	}

	websocketClient := th.websocketClients[userID]
	require.NotNil(t, websocketClient, "websocket client must be setup first by calling SetupWebsocketClientForUser")

	return websocketClient
}

func (th *testHelper) assertEphemeralMessage(t *testing.T, userID, channelID, message string) {
	t.Helper()

	websocketClient := th.GetWebsocketClientForUser(t, userID)

	for {
		select {
		case event, ok := <-websocketClient.EventChannel:
			if !ok {
				t.Fatal("channel closed before getting websocket event for ephemeral message")
			}

			if event.EventType() == model.WebsocketEventEphemeralMessage {
				data := event.GetData()
				postJSON, ok := data["post"].(string)
				require.True(t, ok, "failed to find post in ephemeral message websocket event")

				var post model.Post
				err := json.Unmarshal([]byte(postJSON), &post)
				require.NoError(t, err)

				assert.Equal(t, channelID, post.ChannelId)
				assert.Equal(t, message, post.Message)

				// If we get this far, we're good!
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("failed to get websocket event for ephemeral message")
		}
	}
}

func (th *testHelper) assertNoEphemeralMessage(t *testing.T, userID, channelID, message string, maxWaitTime time.Duration) {
	t.Helper()

	websocketClient := th.GetWebsocketClientForUser(t, userID)

	for {
		select {
		case event, ok := <-websocketClient.EventChannel:
			if !ok {
				t.Fatal("channel closed before getting websocket event for ephemeral message")
			}

			if event.EventType() == model.WebsocketEventEphemeralMessage {
				data := event.GetData()
				postJSON, ok := data["post"].(string)
				require.True(t, ok, "failed to find post in ephemeral message websocket event")

				var post model.Post
				err := json.Unmarshal([]byte(postJSON), &post)
				require.NoError(t, err)

				if post.ChannelId == channelID && post.Message == message {
					t.Fatalf("received undesired ephemeral message in channel %s: %s", post.ChannelId, post.Message)
				}
			}
		case <-time.After(maxWaitTime):
			// Did not get the message, so we're good!
			t.Log("Did not receive undesired ephemeral message")
			return
		}
	}
	return
}

func (th *testHelper) retrieveEphemeralPost(t *testing.T, userID, channelID string) *model.Post {
	t.Helper()

	websocketClient := th.GetWebsocketClientForUser(t, userID)

	for {
		select {
		case event, ok := <-websocketClient.EventChannel:
			if !ok {
				t.Fatal("channel closed before getting websocket event for ephemeral message")
			}

			if event.EventType() == model.WebsocketEventEphemeralMessage {
				data := event.GetData()
				postJSON, ok := data["post"].(string)
				require.True(t, ok, "failed to find post in ephemeral message websocket event")

				var post model.Post
				err := json.Unmarshal([]byte(postJSON), &post)
				require.NoError(t, err)

				assert.Equal(t, channelID, post.ChannelId)
				return &post
			}
		case <-time.After(5 * time.Second):
			t.Fatal("failed to get websocket event for ephemeral message")
		}
	}
}

func (th *testHelper) assertDMFromUser(t *testing.T, fromUserID, toUserID, expectedMessage string) {
	t.Helper()

	channel, appErr := th.p.API.GetDirectChannel(fromUserID, toUserID)
	require.Nil(t, appErr)

	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		postList, appErr := th.p.API.GetPostsSince(channel.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
		require.Nil(t, appErr)

		for _, post := range postList.Posts {
			if post.Message == expectedMessage {
				return
			}
		}
		t.Errorf("failed to find post with expected message: %s", expectedMessage)
	}, 1*time.Second, 10*time.Millisecond)
}

func (th *testHelper) assertNoDMFromUser(t *testing.T, fromUserID, toUserID string) {
	t.Helper()

	channel, appErr := th.p.API.GetDirectChannel(fromUserID, toUserID)
	require.Nil(t, appErr)

	assert.Never(t, func() bool {
		postList, appErr := th.p.API.GetPostsSince(channel.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
		require.Nil(t, appErr)

		return len(postList.Posts) > 0
	}, 1*time.Second, 10*time.Millisecond, "expected no DMs from user")
}
