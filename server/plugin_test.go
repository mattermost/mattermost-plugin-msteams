package main

import (
	"os"
	"path"
	"testing"

	metricsmocks "github.com/mattermost/mattermost-plugin-msteams/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
)

func newTestPlugin(t *testing.T) *Plugin {
	clientMock := &mocks.Client{}
	plugin := &Plugin{
		MattermostPlugin: plugin.MattermostPlugin{
			API:    &plugintest.API{},
			Driver: &plugintest.Driver{},
		},
		configuration: &configuration{
			TenantID:                   "",
			ClientID:                   "",
			ClientSecret:               "",
			WebhookSecret:              "webhooksecret",
			EncryptionKey:              "encryptionkey",
			MaxSizeForCompleteDownload: 1,
		},
		msteamsAppClient: &mocks.Client{},
		store:            &storemocks.Store{},
		clientBuilderWithToken: func(redirectURL, tenantID, clientId, clientSecret string, token *oauth2.Token, apiClient *pluginapi.LogService) msteams.Client {
			return clientMock
		},
		remoteID: "remote-id",
	}
	plugin.store.(*storemocks.Store).On("Shutdown").Return(nil)
	plugin.store.(*storemocks.Store).Test(t)

	plugin.msteamsAppClient.(*mocks.Client).On("ClearSubscriptions").Return(nil)
	plugin.msteamsAppClient.(*mocks.Client).On("RefreshSubscriptionsPeriodically", mock.Anything, mock.Anything).Return(nil)
	plugin.msteamsAppClient.(*mocks.Client).On("SubscribeToChannels", mock.Anything, plugin.configuration.WebhookSecret, "").Return("channel-subscription-id", nil)
	plugin.msteamsAppClient.(*mocks.Client).On("SubscribeToChats", mock.Anything, plugin.configuration.WebhookSecret, "").Return("chats-subscription-id", nil)
	plugin.msteamsAppClient.(*mocks.Client).On("SubscribeToChannel", mock.Anything, mock.Anything, "/plugins/com.mattermost.msteams-sync/", plugin.configuration.WebhookSecret, "").Return(&clientmodels.Subscription{ID: "channel-subscription-id"}, nil)
	plugin.msteamsAppClient.(*mocks.Client).Test(t)
	bot := &model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: "Created by the MS Teams Sync plugin.",
	}
	config := model.Config{}
	config.SetDefaults()
	plugin.API.(*plugintest.API).On("KVGet", "cron_monitoring_system").Return(nil, nil).Maybe()
	plugin.API.(*plugintest.API).On("GetServerVersion").Return("7.8.0")
	plugin.API.(*plugintest.API).On("GetBundlePath").Return("./dist", nil).Maybe()
	plugin.API.(*plugintest.API).On("Conn", true).Return("connection-id", nil)
	plugin.API.(*plugintest.API).On("GetUnsanitizedConfig").Return(&config)
	plugin.API.(*plugintest.API).On("EnsureBotUser", bot).Return("bot-user-id", nil).Times(1)
	plugin.API.(*plugintest.API).On("SetProfileImage", mock.AnythingOfType("string"), mock.AnythingOfType("[]uint8")).Return(nil).Times(1)
	plugin.API.(*plugintest.API).On("RegisterCommand", mock.Anything).Return(nil).Times(1)
	plugin.API.(*plugintest.API).On("KVList", 0, 1000000000).Return([]string{}, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_cron_monitoring_system", []byte{0x1}, mock.Anything).Return(true, nil).Maybe()
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "cron_monitoring_system", mock.Anything, model.PluginKVSetOptions{ExpireInSeconds: 0}).Return(true, nil).Maybe()
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_cron_monitoring_system", []byte(nil), mock.Anything).Return(true, nil).Maybe()
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_subscriptions_cluster_mutex", []byte{0x1}, model.PluginKVSetOptions{Atomic: true, ExpireInSeconds: 15}).Return(true, nil).Maybe()
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_subscriptions_cluster_mutex", []byte(nil), model.PluginKVSetOptions{ExpireInSeconds: 0}).Return(true, nil).Maybe()
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_mmi_bot_ensure", []byte{0x1}, model.PluginKVSetOptions{Atomic: true, ExpireInSeconds: 15}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_mmi_bot_ensure", []byte(nil), model.PluginKVSetOptions{ExpireInSeconds: 0}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVGet", "cron_check_credentials").Return(nil, nil).Maybe()
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_cron_check_credentials", mock.Anything, mock.Anything).Return(true, nil).Maybe()
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "cron_check_credentials", mock.Anything, model.PluginKVSetOptions{ExpireInSeconds: 0}).Return(true, nil).Maybe()
	plugin.API.(*plugintest.API).On("GetLicense").Return(&model.License{SkuShortName: "enterprise"}).Maybe()
	plugin.API.(*plugintest.API).On("GetConfig").Return(&model.Config{
		ServiceSettings: model.ServiceSettings{
			SiteURL: model.NewString("/"),
		},
		FileSettings: model.FileSettings{
			MaxFileSize: model.NewInt64(5),
		},
	}, nil).Maybe()
	plugin.API.(*plugintest.API).On("GetPluginStatus", pluginID).Return(&model.PluginStatus{PluginId: pluginID, PluginPath: getPluginPathForTest()}, nil)
	// TODO: Add separate mocks for each test later.
	mockMetricsService := &metricsmocks.Metrics{}
	mockMetricsService.On("IncrementHTTPRequests")
	mockMetricsService.On("ObserveAPIEndpointDuration", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("float64"))
	testutils.MockLogs(plugin.API.(*plugintest.API))

	plugin.API.(*plugintest.API).Test(t)
	_ = plugin.OnActivate()
	// OnActivate is actually failing right now, but mocking it is quite difficult. So just
	// manually wire up the API by hand until we get the E2E tests going.
	plugin.apiHandler = NewAPI(plugin, plugin.store)

	plugin.metricsService = mockMetricsService
	plugin.botUserID = "bot-user-id"
	return plugin
}

func getPluginPathForTest() string {
	curr, err := os.Getwd()
	if err != nil {
		return ""
	}
	path := path.Join(curr, "..")
	return path
}

func TestGetURL(t *testing.T) {
	testCases := []struct {
		Name     string
		URL      string
		Expected string
	}{
		{
			Name:     "no subpath, ending with /",
			URL:      "https://example.com/",
			Expected: "https://example.com/plugins/" + pluginID,
		},
		{
			Name:     "no subpath, not ending with /",
			URL:      "https://example.com",
			Expected: "https://example.com/plugins/" + pluginID,
		},
		{
			Name:     "with subpath, ending with /",
			URL:      "https://example.com/subpath/",
			Expected: "https://example.com/subpath/plugins/" + pluginID,
		},
		{
			Name:     "with subpath, not ending with /",
			URL:      "https://example.com/subpath",
			Expected: "https://example.com/subpath/plugins/" + pluginID,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			config := &model.Config{}
			config.SetDefaults()
			config.ServiceSettings.SiteURL = model.NewString(testCase.URL)

			actual := getURL(config)
			assert.Equal(t, testCase.Expected, actual)
		})
	}
}

func TestGetRelativeURL(t *testing.T) {
	testCases := []struct {
		Name     string
		URL      string
		Expected string
	}{
		{
			Name:     "Empty URL",
			URL:      "",
			Expected: "/plugins/" + pluginID,
		},
		{
			Name:     "no subpath, ending with /",
			URL:      "https://example.com/",
			Expected: "/plugins/" + pluginID,
		},
		{
			Name:     "no subpath, not ending with /",
			URL:      "https://example.com",
			Expected: "/plugins/" + pluginID,
		},
		{
			Name:     "with subpath, ending with /",
			URL:      "https://example.com/subpath/",
			Expected: "/subpath/plugins/" + pluginID,
		},
		{
			Name:     "with subpath, not ending with /",
			URL:      "https://example.com/subpath",
			Expected: "/subpath/plugins/" + pluginID,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			config := &model.Config{}
			config.SetDefaults()
			config.ServiceSettings.SiteURL = model.NewString(testCase.URL)

			actual := getRelativeURL(config)
			assert.Equal(t, testCase.Expected, actual)
		})
	}
}

func TestGetClientForUser(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("no such user", func(t *testing.T) {
		th.Reset(t)

		client, err := th.p.GetClientForUser("unknown")
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("user never connected", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)

		client, err := th.p.GetClientForUser(user.Id)
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("user disconnected", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		th.ConnectUser(t, user.Id)
		th.DisconnectUser(t, user.Id)

		client, err := th.p.GetClientForUser(user.Id)
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("user connected", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		th.ConnectUser(t, user.Id)

		client, err := th.p.GetClientForUser(user.Id)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})
}

func TestGetClientForTeamsUser(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("no such user", func(t *testing.T) {
		th.Reset(t)

		client, err := th.p.GetClientForTeamsUser("unknown")
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("user never connected", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)

		client, err := th.p.GetClientForTeamsUser("t" + user.Id)
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("user disconnected", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		th.ConnectUser(t, user.Id)
		th.DisconnectUser(t, user.Id)

		client, err := th.p.GetClientForTeamsUser("t" + user.Id)
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("user connected", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		th.ConnectUser(t, user.Id)

		client, err := th.p.GetClientForTeamsUser("t" + user.Id)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})
}

func TestSyncUsers(t *testing.T) {
	t.Skip("Not yet implemented")
}
