package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"testing"
	"time"

	goPlugin "github.com/hashicorp/go-plugin"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type testHelper struct {
	p                        *Plugin
	appClientMock            *mocks.Client
	clientMock               *mocks.Client
	websocketClients         map[string]*model.WebSocketClient
	websocketEventsWhitelist map[string][]model.WebsocketEventType
	metricsSnapshot          []*dto.MetricFamily
}

func setupTestHelper(t *testing.T) *testHelper {
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
					"webhooksecret":           "webhooksecret",
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

	var err error
	th.metricsSnapshot, err = th.p.metricsService.GetRegistry().Gather()
	require.NoError(t, err)

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
					if slices.Contains(th.websocketEventsWhitelist[userID], event.EventType()) {
						// Ignore whitelisted events.
						continue
					}
					unmatchedEvents[userID] = append(unmatchedEvents[userID], event)
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

func (th *testHelper) LinkChannel(t *testing.T, team *model.Team, channel *model.Channel, user *model.User) *storemodels.ChannelLink {
	channelLink := storemodels.ChannelLink{
		MattermostTeamID:    team.Id,
		MattermostChannelID: channel.Id,
		MSTeamsTeam:         model.NewId(),
		MSTeamsChannel:      model.NewId(),
		Creator:             user.Id,
	}
	err := th.p.store.StoreChannelLink(&channelLink)
	require.NoError(t, err)

	return &channelLink
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

	err := th.p.store.SetUserInfo(user.Id, "t"+user.Id, nil)
	require.NoError(t, err)

	return user
}

func (th *testHelper) SetupGuestUser(t *testing.T, team *model.Team) *model.User {
	t.Helper()

	var user *model.User
	var err error
	user = th.SetupUser(t, team)

	user, err = th.p.apiClient.User.UpdateRoles(user.Id, model.SystemGuestRoleId)
	require.NoError(t, err)

	return user
}

func (th *testHelper) CreateBot(t *testing.T) *model.Bot {
	id := model.NewId()

	bot := &model.Bot{
		Username:    "bot" + id,
		DisplayName: "a bot",
		Description: "bot",
		OwnerId:     "ownerID",
	}

	bot, err := th.p.API.CreateBot(bot)
	require.Nil(t, err)

	return bot
}

func (th *testHelper) ConnectUser(t *testing.T, userID string) {
	teamID := "t" + userID
	err := th.p.store.SetUserInfo(userID, teamID, &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
	require.NoError(t, err)
}

func (th *testHelper) DisconnectUser(t *testing.T, userID string) {
	teamID := "t" + userID
	err := th.p.store.SetUserInfo(userID, teamID, nil)
	require.NoError(t, err)
}

func (th *testHelper) MarkUserInvited(t *testing.T, userID string) {
	t.Helper()
	invitedUser := &storemodels.InvitedUser{ID: userID, InvitePendingSince: time.Now(), InviteLastSentAt: time.Now()}
	err := th.p.GetStore().StoreInvitedUser(invitedUser)
	assert.NoError(t, err)
}

func (th *testHelper) MarkUserWhitelisted(t *testing.T, userID string) {
	t.Helper()
	err := th.p.store.StoreUserInWhitelist(userID)
	assert.NoError(t, err)
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

func (th *testHelper) pluginURL(t *testing.T, paths ...string) string {
	baseURL, err := url.JoinPath(os.Getenv("MM_SERVICESETTINGS_SITEURL"), "plugins", pluginID)
	require.NoError(t, err)

	apiURL, err := url.JoinPath(baseURL, paths...)
	require.NoError(t, err)

	return apiURL
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
func (th *testHelper) SetupWebsocketClientForUser(t *testing.T, userID string, whitelistedEvents ...model.WebsocketEventType) {
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

	if th.websocketEventsWhitelist == nil {
		th.websocketEventsWhitelist = make(map[string][]model.WebsocketEventType)
	}

	// Ignore these common events by default.
	th.websocketEventsWhitelist[userID] = append(th.websocketEventsWhitelist[userID], model.WebsocketEventHello)
	th.websocketEventsWhitelist[userID] = append(th.websocketEventsWhitelist[userID], model.WebsocketEventStatusChange)

	th.websocketEventsWhitelist[userID] = append(th.websocketEventsWhitelist[userID], whitelistedEvents...)
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
	require.NotNil(t, websocketClient, "websocket client must be setup first")

	return websocketClient
}

func makePluginWebsocketEventName(short string) string {
	return fmt.Sprintf("custom_%s_%s", manifest.Id, short)
}

func (th *testHelper) assertWebsocketEvent(t *testing.T, userID, eventType string) {
	t.Helper()

	websocketClient := th.GetWebsocketClientForUser(t, userID)

	for {
		select {
		case event, ok := <-websocketClient.EventChannel:
			if !ok {
				t.Fatal("channel closed before getting websocket event")
			}

			if event.EventType() == model.WebsocketEventType(eventType) {
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("failed to get websocket event " + eventType)
		}
	}
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

func (th *testHelper) assertGMFromUsers(t *testing.T, fromUserID string, otherUsers []string, expectedMessage string) {
	t.Helper()

	var users []string
	users = append(users, fromUserID)
	users = append(users, otherUsers...)

	channel, appErr := th.p.API.GetGroupChannel(users)
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

func (th *testHelper) assertDMFromUserRe(t *testing.T, fromUserID, toUserID, expectedMessageRe string) {
	t.Helper()

	channel, appErr := th.p.API.GetDirectChannel(fromUserID, toUserID)
	require.Nil(t, appErr)

	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		postList, appErr := th.p.API.GetPostsSince(channel.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
		require.Nil(t, appErr)

		for _, post := range postList.Posts {
			matched, err := regexp.MatchString(expectedMessageRe, post.Message)
			require.NoError(t, err)
			if matched {
				return
			}
		}
		t.Errorf("failed to find post matching expected message re: %s", expectedMessageRe)
	}, 1*time.Second, 10*time.Millisecond)
}

func (th *testHelper) assertNoDMFromUser(t *testing.T, fromUserID, toUserID string, checkTime int64) {
	t.Helper()

	channel, appErr := th.p.API.GetDirectChannel(fromUserID, toUserID)
	require.Nil(t, appErr)

	assert.Never(t, func() bool {
		postList, appErr := th.p.API.GetPostsSince(channel.Id, checkTime)
		require.Nil(t, appErr)

		return len(postList.Posts) > 0
	}, 1*time.Second, 10*time.Millisecond, "expected no DMs from user")
}

func (th *testHelper) assertNoGMFromUsers(t *testing.T, fromUserID string, otherUsers []string, checkTime int64) {
	t.Helper()

	var users []string
	users = append(users, fromUserID)
	users = append(users, otherUsers...)

	channel, appErr := th.p.API.GetGroupChannel(users)
	require.Nil(t, appErr)

	assert.Never(t, func() bool {
		postList, appErr := th.p.API.GetPostsSince(channel.Id, checkTime)
		require.Nil(t, appErr)

		return len(postList.Posts) > 0
	}, 1*time.Second, 10*time.Millisecond, "expected no DMs from user")
}

func (th *testHelper) assertPostInChannel(t *testing.T, fromUserID, channelID, expectedMessage string) {
	t.Helper()

	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		postList, appErr := th.p.API.GetPostsSince(channelID, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
		require.Nil(t, appErr)

		for _, post := range postList.Posts {
			if post.UserId == fromUserID && post.Message == expectedMessage {
				return
			}
		}
		t.Errorf("failed to find post with expected message: %s", expectedMessage)
	}, 1*time.Second, 10*time.Millisecond)
}

func (th *testHelper) assertNoPostInChannel(t *testing.T, fromUserID, channelID string) {
	t.Helper()

	assert.Never(t, func() bool {
		postList, appErr := th.p.API.GetPostsSince(channelID, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
		require.Nil(t, appErr)

		for _, post := range postList.Posts {
			if post.UserId == fromUserID {
				return true
			}
		}

		return false
	}, 1*time.Second, 10*time.Millisecond)
}

type labelOptionFunc func(metric *dto.Metric) bool

func withLabel(name, value string) labelOptionFunc {
	return func(metric *dto.Metric) bool {
		for _, label := range metric.Label {
			if *label.Name == name && *label.Value == value {
				return true
			}
		}

		return false
	}
}

// getRelativeMetric returns the value of the given counter relative to the start of the test.
func (th *testHelper) getRelativeCounter(t *testing.T, name string, labelOptions ...labelOptionFunc) float64 {
	getCounterValue := func(metricFamilies []*dto.MetricFamily, name string) float64 {
		for _, metricFamily := range metricFamilies {
			if *metricFamily.Name != name {
				continue
			}

		nextMetric:
			for _, metric := range metricFamily.Metric {
				for _, labelOption := range labelOptions {
					if !labelOption(metric) {
						continue nextMetric
					}
				}

				return *metric.Counter.Value
			}
		}

		return 0
	}

	currentMetricFamilies, err := th.p.metricsService.GetRegistry().Gather()
	require.NoError(t, err)

	before := getCounterValue(th.metricsSnapshot, name)
	after := getCounterValue(currentMetricFamilies, name)

	return after - before
}

func (th *testHelper) setPluginConfiguration(t *testing.T, update func(configuration *configuration)) (*configuration, *configuration) {
	t.Helper()

	c := th.p.getConfiguration().Clone()
	prev := c.Clone()

	update(c)
	th.p.setConfiguration(c)

	return c, prev
}

func (th *testHelper) setPluginConfigurationTemporarily(t *testing.T, update func(configuration *configuration)) {
	t.Helper()

	_, prev := th.setPluginConfiguration(t, func(config *configuration) { update(config) })

	t.Cleanup(func() {
		th.p.setConfiguration(prev)
	})
}
