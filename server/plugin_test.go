package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	metricsmocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	sqlstore "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/sqlstore"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils/mmcontainer"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
)

func newE2ETestPlugin(t *testing.T) (*mmcontainer.MattermostContainer, *sqlstore.SQLStore, func(ctx context.Context) error) {
	ctx := context.Background()
	matches, err := filepath.Glob("../dist/*.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("Unable to find plugin tar.gz file")
	}
	filename := matches[0]

	pluginConfig := map[string]any{
		"clientid":                   "client-id",
		"clientsecret":               "client-secret",
		"connectedusersallowed":      1000,
		"encryptionkey":              "eyPBz0mBhwfGGwce9hp4TWaYzgY7MdIB",
		"maxSizeForCompleteDownload": 20,
		"maxsizeforcompletedownload": 20,
		"tenantid":                   "tenant-id",
		"webhooksecret":              "webhook-secret",
	}
	mattermost, err := mmcontainer.RunContainer(ctx,
		mmcontainer.WithPlugin("../dist/"+filename, "com.mattermost.msteams-sync", pluginConfig),
		mmcontainer.WithEnv("MM_MSTEAMSSYNC_MOCK_CLIENT", "true"),
	)

	conn, err := mattermost.PostgresConnection(ctx)
	if err != nil {
		mattermost.Terminate(ctx)
	}
	require.NoError(t, err)

	store := sqlstore.New(conn, "postgres", nil, func() []string { return []string{""} }, func() []byte { return []byte("eyPBz0mBhwfGGwce9hp4TWaYzgY7MdIB") })
	store.Init()

	return mattermost, store, mattermost.Terminate
}

func mockMSTeamsClient(t *testing.T, client *model.Client4, method string, returnType string, returns interface{}, returnErr string) {
	mockStruct := MockCallReturns{ReturnType: returnType, Returns: returns, Err: returnErr}
	mockData, err := json.Marshal(mockStruct)
	require.NoError(t, err)

	_, err = client.DoAPIRequest(context.Background(), http.MethodPost, client.URL+"/plugins/com.mattermost.msteams-sync/add-mock/"+method, string(mockData), "")
	require.NoError(t, err)
}

func newTestPlugin(t *testing.T) *Plugin {
	clientMock := &mocks.Client{}
	plugin := &Plugin{
		MattermostPlugin: plugin.MattermostPlugin{
			API:    &plugintest.API{},
			Driver: &plugintest.Driver{},
		},
		configuration: &configuration{
			TenantID:          "",
			ClientID:          "",
			ClientSecret:      "",
			WebhookSecret:     "webhooksecret",
			EncryptionKey:     "encryptionkey",
			CertificatePublic: "",
			CertificateKey:    "",
		},
		msteamsAppClient: &mocks.Client{},
		store:            &storemocks.Store{},
		clientBuilderWithToken: func(redirectURL, tenantID, clientId, clientSecret string, token *oauth2.Token, apiClient *pluginapi.LogService) msteams.Client {
			return clientMock
		},
	}
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
	plugin.API.(*plugintest.API).On("KVGet", "cron_monitoring_system").Return(nil, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetServerVersion").Return("7.8.0")
	plugin.API.(*plugintest.API).On("GetBundlePath").Return("./dist", nil)
	plugin.API.(*plugintest.API).On("Conn", true).Return("connection-id", nil)
	plugin.API.(*plugintest.API).On("GetUnsanitizedConfig").Return(&config)
	plugin.API.(*plugintest.API).On("EnsureBotUser", bot).Return("bot-user-id", nil).Times(1)
	plugin.API.(*plugintest.API).On("RegisterCommand", mock.Anything).Return(nil).Times(1)
	plugin.API.(*plugintest.API).On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	plugin.API.(*plugintest.API).On("KVList", 0, 1000000000).Return([]string{}, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_cron_monitoring_system", []byte{0x1}, model.PluginKVSetOptions{Atomic: true, ExpireInSeconds: 15}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "cron_monitoring_system", mock.Anything, model.PluginKVSetOptions{ExpireInSeconds: 0}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_cron_monitoring_system", []byte(nil), model.PluginKVSetOptions{ExpireInSeconds: 0}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_subscriptions_cluster_mutex", []byte{0x1}, model.PluginKVSetOptions{Atomic: true, ExpireInSeconds: 15}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_subscriptions_cluster_mutex", []byte(nil), model.PluginKVSetOptions{ExpireInSeconds: 0}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_mmi_bot_ensure", []byte{0x1}, model.PluginKVSetOptions{Atomic: true, ExpireInSeconds: 15}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_mmi_bot_ensure", []byte(nil), model.PluginKVSetOptions{ExpireInSeconds: 0}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
	plugin.API.(*plugintest.API).On("GetPluginStatus", pluginID).Return(&model.PluginStatus{PluginId: pluginID, PluginPath: getPluginPathForTest()}, nil)
	// TODO: Add separate mocks for each test later.
	mockMetricsService := &metricsmocks.Metrics{}
	mockMetricsService.On("IncrementHTTPRequests")
	mockMetricsService.On("ObserveAPIEndpointDuration", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("float64"))

	plugin.API.(*plugintest.API).Test(t)
	_ = plugin.OnActivate()
	plugin.metricsService = mockMetricsService
	plugin.userID = "bot-user-id"
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

func TestMessageHasBeenPostedNewMessageE2E(t *testing.T) {
	t.Parallel()
	mattermost, store, tearDown := newE2ETestPlugin(t)
	defer tearDown(context.Background())

	client, err := mattermost.GetAdminClient(context.Background())
	require.NoError(t, err)

	user, _, err := client.GetMe(context.Background(), "")
	require.NoError(t, err)

	team, _, err := client.GetTeamByName(context.Background(), "test", "")
	require.NoError(t, err)

	channel, _, err := client.GetChannelByName(context.Background(), "town-square", team.Id, "")
	require.NoError(t, err)

	post := model.Post{
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
		UserId:    user.Id,
		ChannelId: channel.Id,
		Message:   "message",
	}

	err = store.SetUserInfo(user.Id, "ms-user-id", &oauth2.Token{})
	require.NoError(t, err)

	mockMSTeamsClient(t, client, "GetChannelInTeam", "Channel", clientmodels.Channel{ID: "ms-channel-id"}, "")

	_, _, err = client.ExecuteCommand(context.Background(), channel.Id, "/msteams-sync link ms-team-id ms-channel-id")
	require.NoError(t, err)

	mockMSTeamsClient(t, client, "SendMessageWithAttachments", "Message", clientmodels.Message{ID: "ms-post-id", LastUpdateAt: time.Now()}, "")

	newPost, _, err := client.CreatePost(context.Background(), &post)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	postInfo, err := store.GetPostInfoByMattermostID(newPost.Id)
	require.NoError(t, err)
	require.Equal(t, postInfo.MSTeamsID, "ms-post-id")
}

func TestMessageHasBeenPostedNewMessageWithoutChannelLinkE2E(t *testing.T) {
	t.Parallel()
	mattermost, store, tearDown := newE2ETestPlugin(t)
	defer tearDown(context.Background())

	client, err := mattermost.GetAdminClient(context.Background())
	require.NoError(t, err)

	user, _, err := client.GetMe(context.Background(), "")
	require.NoError(t, err)

	team, _, err := client.GetTeamByName(context.Background(), "test", "")
	require.NoError(t, err)

	channel, _, err := client.GetChannelByName(context.Background(), "town-square", team.Id, "")
	require.NoError(t, err)

	post := model.Post{
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
		UserId:    user.Id,
		ChannelId: channel.Id,
		Message:   "message",
	}

	newPost, _, err := client.CreatePost(context.Background(), &post)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	_, err = store.GetPostInfoByMattermostID(newPost.Id)
	require.Error(t, err)
}

func TestMessageHasBeenPostedNewMessageWithFailureSendingE2E(t *testing.T) {
	t.Parallel()
	mattermost, store, tearDown := newE2ETestPlugin(t)
	defer tearDown(context.Background())

	client, err := mattermost.GetAdminClient(context.Background())
	require.NoError(t, err)

	user, _, err := client.GetMe(context.Background(), "")
	require.NoError(t, err)

	team, _, err := client.GetTeamByName(context.Background(), "test", "")
	require.NoError(t, err)

	channel, _, err := client.GetChannelByName(context.Background(), "town-square", team.Id, "")
	require.NoError(t, err)

	post := model.Post{
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
		UserId:    user.Id,
		ChannelId: channel.Id,
		Message:   "message",
	}

	err = store.SetUserInfo(user.Id, "ms-user-id", &oauth2.Token{})
	require.NoError(t, err)

	mockMSTeamsClient(t, client, "GetChannelInTeam", "Channel", clientmodels.Channel{ID: "ms-channel-id"}, "")

	_, _, err = client.ExecuteCommand(context.Background(), channel.Id, "/msteams-sync link ms-team-id ms-channel-id")
	require.NoError(t, err)

	mockMSTeamsClient(t, client, "SendMessageWithAttachments", "Message", nil, "Unable to send the message")

	newPost, _, err := client.CreatePost(context.Background(), &post)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	logs, err := mattermost.GetLogs(context.Background(), 10)
	require.NoError(t, err)

	require.Contains(t, logs, "Error creating post on MS Teams")
	require.Contains(t, logs, "Unable to handle message sent")

	_, err = store.GetPostInfoByMattermostID(newPost.Id)
	require.Error(t, err)
}

func TestGetURL(t *testing.T) {
	mockSiteURLWithSuffix := "mockSiteURL/"
	mockSiteURLWithoutSuffix := "mockSiteURL"
	for _, test := range []struct {
		Name     string
		SetupAPI func(*plugintest.API)
	}{
		{
			Name: "GetURL: With suffix '/'",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &mockSiteURLWithSuffix,
					},
				}).Times(1)
			},
		},
		{
			Name: "GetURL: Without suffix '/'",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &mockSiteURLWithoutSuffix,
					},
				}).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			apiMock := &plugintest.API{}
			test.SetupAPI(apiMock)
			p.SetAPI(apiMock)
			resp := p.GetURL()
			assert.Equal("mockSiteURL/plugins/com.mattermost.msteams-sync", resp)
		})
	}
}

func TestGetClientForUser(t *testing.T) {
	for _, test := range []struct {
		Name          string
		SetupStore    func(*storemocks.Store)
		ExpectedError string
	}{
		{
			Name: "GetClientForUser: Unable to get the token",
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			ExpectedError: "not connected user",
		},
		{
			Name: "GetClientForUser: Valid",
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupStore(p.store.(*storemocks.Store))
			resp, err := p.GetClientForUser(testutils.GetID())
			if test.ExpectedError != "" {
				assert.Nil(resp)
				assert.EqualError(err, test.ExpectedError)
			} else {
				assert.Nil(err)
				assert.NotNil(resp)
			}
		})
	}
}

func TestGetClientForTeamsUser(t *testing.T) {
	for _, test := range []struct {
		Name          string
		SetupStore    func(*storemocks.Store)
		ExpectedError string
	}{
		{
			Name: "GetClientForTeamsUser: Unable to get the token",
			SetupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, nil).Times(1)
			},
			ExpectedError: "not connected user",
		},
		{
			Name: "GetClientForTeamsUser: Valid",
			SetupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupStore(p.store.(*storemocks.Store))
			resp, err := p.GetClientForTeamsUser(testutils.GetTeamsUserID())
			if test.ExpectedError != "" {
				assert.Nil(resp)
				assert.EqualError(err, test.ExpectedError)
			} else {
				assert.Nil(err)
				assert.NotNil(resp)
			}
		})
	}
}

func TestSyncUsers(t *testing.T) {
	for _, test := range []struct {
		Name         string
		SetupAPI     func(*plugintest.API)
		SetupStore   func(*storemocks.Store)
		SetupClient  func(*mocks.Client)
		SetupMetrics func(*metricsmocks.Metrics)
	}{
		{
			Name: "SyncUsers: Unable to get the MS Teams user list",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to list MS Teams users during sync user job", "error", mock.Anything).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {},
			SetupClient: func(client *mocks.Client) {
				client.On("ListUsers").Return(nil, errors.New("unable to get the user list")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {},
		},
		{
			Name: "SyncUsers: Unable to get the MM users",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get MM users during sync user job", "error", mock.Anything).Times(1)
				api.On("GetUsers", &model.UserGetOptions{
					Page:    0,
					PerPage: math.MaxInt32,
				}).Return(nil, testutils.GetInternalServerAppError("unable to get the users")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {},
			SetupClient: func(client *mocks.Client) {
				client.On("ListUsers").Return([]clientmodels.User{
					{
						ID:          testutils.GetTeamsUserID(),
						DisplayName: "mockDisplayName",
					},
				}, nil).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveUpstreamUsers", int64(1)).Times(1)
			},
		},
		{
			Name: "SyncUsers: Unable to create the user",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to create new MM user during sync job", "MMUserID", mock.Anything, "TeamsUserID", mock.Anything, "error", mock.Anything).Times(1)
				api.On("GetUsers", &model.UserGetOptions{
					Page:    0,
					PerPage: math.MaxInt32,
				}).Return([]*model.User{
					testutils.GetUser(model.SystemAdminRoleId, "test@test.com"),
				}, nil).Times(1)
				api.On("CreateUser", mock.AnythingOfType("*model.User")).Return(nil, testutils.GetInternalServerAppError("unable to create the user")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {},
			SetupClient: func(client *mocks.Client) {
				client.On("ListUsers").Return([]clientmodels.User{
					{
						ID:          testutils.GetTeamsUserID(),
						DisplayName: "mockDisplayName",
					},
				}, nil).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveUpstreamUsers", int64(1)).Times(1)
			},
		},
		{
			Name: "SyncUsers: Unable to store the user info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to set user info during sync user job", "MMUserID", mock.Anything, "TeamsUserID", mock.Anything, "error", mock.Anything).Times(1)
				api.On("GetUsers", &model.UserGetOptions{
					Page:    0,
					PerPage: math.MaxInt32,
				}).Return([]*model.User{
					testutils.GetUser(model.SystemAdminRoleId, "test@test.com"),
				}, nil).Times(1)
				api.On("CreateUser", mock.AnythingOfType("*model.User")).Return(&model.User{
					Id: testutils.GetID(),
				}, nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("SetUserInfo", testutils.GetID(), testutils.GetTeamsUserID(), mock.AnythingOfType("*oauth2.Token")).Return(testutils.GetInternalServerAppError("unable to store the user info")).Times(1)
			},
			SetupClient: func(client *mocks.Client) {
				client.On("ListUsers").Return([]clientmodels.User{
					{
						ID:          testutils.GetTeamsUserID(),
						DisplayName: "mockDisplayName",
					},
				}, nil).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveUpstreamUsers", int64(1)).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*mocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			p.syncUsers()
		})
	}
}

func TestStart(t *testing.T) {
	mockSiteURL := "mockSiteURL"
	for _, test := range []struct {
		Name        string
		IsRestart   bool
		SetupAPI    func(*plugintest.API)
		SetupClient func(*mocks.Client)
		SetupStore  func(*storemocks.Store)
	}{
		{
			Name:      "Start: Valid",
			IsRestart: false,
			SetupAPI: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &mockSiteURL,
					},
				})
				api.On("LogError", "Unable to start the monitoring system", "error", "error in setting job status").Return()
			},
			SetupClient: func(client *mocks.Client) {
				client.On("Connect").Return(nil).Times(1)
			},
			SetupStore: func(s *storemocks.Store) {
				s.On("SetJobStatus", "monitoring_system", false).Return(errors.New("error in setting job status"))
				s.On("CompareAndSetJobStatus", "monitoring_system", false, true).Return(false, nil)
				s.On("DeleteFakeSubscriptions").Return(nil).Times(1)
				s.On("GetSubscriptionsLastActivityAt").Return(map[string]time.Time{}, nil)
			},
		},
		{
			Name:      "Restart: Valid",
			IsRestart: true,
			SetupAPI: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &mockSiteURL,
					},
				})
				api.On("LogError", "Unable to start the monitoring system", "error", "error in setting job status").Return()
			},
			SetupClient: func(client *mocks.Client) {
				client.On("Connect").Return(nil).Times(1)
			},
			SetupStore: func(s *storemocks.Store) {
				s.On("GetSubscriptionsLastActivityAt").Return(map[string]time.Time{}, nil)
				s.On("SetJobStatus", "monitoring_system", false).Return(errors.New("error in setting job status"))
				s.On("CompareAndSetJobStatus", "monitoring_system", false, true).Return(false, nil)
				s.On("DeleteFakeSubscriptions").Return(nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			p.metricsService.(*metricsmocks.Metrics).On("ObserveChangeEventQueueCapacity", int64(5000)).Times(1)
			subscriptionsMutex, err := cluster.NewMutex(p.API, subscriptionsClusterMutexKey)
			require.Nil(t, err)
			whitelistMutex, err := cluster.NewMutex(p.API, whitelistClusterMutexKey)
			require.Nil(t, err)
			p.subscriptionsClusterMutex = subscriptionsMutex
			p.whitelistClusterMutex = whitelistMutex
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupClient(p.msteamsAppClient.(*mocks.Client))
			test.SetupStore(p.store.(*storemocks.Store))
			p.start(test.IsRestart)
			time.Sleep(5 * time.Second)
		})
	}
}

func TestGeneratePluginSecrets(t *testing.T) {
	for _, test := range []struct {
		Name          string
		SetupAPI      func(*plugintest.API)
		ExpectedError string
	}{
		{
			Name: "GeneratePluginSecrets: Unable to save plugin config",
			SetupAPI: func(api *plugintest.API) {
				api.On("SavePluginConfig", mock.Anything).Return(testutils.GetInternalServerAppError("unable to save plugin config")).Times(1)
			},
			ExpectedError: "unable to save plugin config",
		},
		{
			Name: "GeneratePluginSecrets: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("SavePluginConfig", mock.Anything).Return(nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			p.configuration.WebhookSecret = ""
			p.configuration.EncryptionKey = ""
			test.SetupAPI(p.API.(*plugintest.API))
			err := p.generatePluginSecrets()
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}
