package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	goPlugin "github.com/hashicorp/go-plugin"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type testHelper struct {
	p             *Plugin
	appClientMock *mocks.Client
	clientMock    *mocks.Client
}

func setupTestHelper(t *testing.T) *testHelper {
	t.Helper()

	appClientMock := &mocks.Client{}
	clientMock := &mocks.Client{}

	p := &Plugin{
		msteamsAppClient: appClientMock,
		clientBuilderWithToken: func(redirectURL, tenantID, clientId, clientSecret string, token *oauth2.Token, apiClient *pluginapi.LogService) msteams.Client {
			return clientMock
		},
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
		p,
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
					"tenantid":      model.NewId(),
					"clientid":      model.NewId(),
					"clientsecret":  model.NewId(),
					"encryptionkey": "aaaaaaaaaaaaaaaaaaaaaaaaaaaa_aaa",
					"webhooksecret": model.NewId(),
					"syncusers":     0,
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

	return &testHelper{p, appClientMock, clientMock}
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
	_, err = db.Exec("DELETE FROM msteamssync_whitelisted_users")
	require.NoError(t, err)
}

func (th *testHelper) Reset(t *testing.T) *testHelper {
	t.Helper()

	// Wipe the tables for this plugin to ensure a clean slate. Note that we don't currently
	// touch any Mattermost tables.
	th.clearDatabase(t)

	th.appClientMock = &mocks.Client{}
	th.clientMock = &mocks.Client{}
	th.appClientMock.Test(t)
	th.clientMock.Test(t)

	th.p.msteamsAppClient = th.appClientMock
	th.p.clientBuilderWithToken = func(redirectURL, tenantID, clientId, clientSecret string, token *oauth2.Token, apiClient *pluginapi.LogService) msteams.Client {
		return th.clientMock
	}

	t.Cleanup(func() {
		th.appClientMock.AssertExpectations(t)
		th.clientMock.AssertExpectations(t)
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

func (th *testHelper) SetupPublicChannel(t *testing.T, team *model.Team) *model.Channel {
	t.Helper()

	channelName := model.NewId()
	channel, appErr := th.p.API.CreateChannel(&model.Channel{
		Name:        channelName,
		DisplayName: channelName,
		Type:        model.ChannelTypeOpen,
		TeamId:      team.Id,
	})
	require.Nil(t, appErr)

	return channel
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

func (th *testHelper) SetupSysadmin(t *testing.T, team *model.Team) *model.User {
	t.Helper()

	var user *model.User
	var err error
	user = th.SetupUser(t, team)

	user, err = th.p.apiClient.User.UpdateRoles(user.Id, model.SystemUserRoleId+" "+model.SystemAdminRoleId)
	require.NoError(t, err)

	return user
}

func (th *testHelper) SetupClient(t *testing.T, user *model.User) *model.Client4 {
	t.Helper()

	// TODO: Don't hard-code this!

	client := model.NewAPIv4Client(os.Getenv("MM_SERVICESETTINGS_SITEURL"))

	// TODO: Don't hardcode "password"
	_, _, err := client.Login(context.TODO(), user.Username, "password")
	require.NoError(t, err)

	return client
}

func (th *testHelper) SetupWebsocketClient(t *testing.T, client *model.Client4) *model.WebSocketClient {
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
