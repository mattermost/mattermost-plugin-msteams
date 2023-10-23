package main

import (
	"database/sql"
	"math"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/cluster"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	metricsmocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
)

func newTestPlugin(t *testing.T) *Plugin {
	clientMock := &mocks.Client{}
	plugin := &Plugin{
		MattermostPlugin: plugin.MattermostPlugin{
			API:    &plugintest.API{},
			Driver: &plugintest.Driver{},
		},
		configuration: &configuration{
			TenantID:      "",
			ClientID:      "",
			ClientSecret:  "",
			WebhookSecret: "webhooksecret",
			EncryptionKey: "encryptionkey",
		},
		msteamsAppClient: &mocks.Client{},
		store:            &storemocks.Store{},
		clientBuilderWithToken: func(redirectURL, tenantID, clientId, clientSecret string, token *oauth2.Token, apiClient *pluginapi.LogService, metricsService metrics.Metrics) msteams.Client {
			return clientMock
		},
		metricsService: &metricsmocks.Metrics{},
	}
	plugin.store.(*storemocks.Store).Test(t)

	plugin.msteamsAppClient.(*mocks.Client).On("ClearSubscriptions").Return(nil)
	plugin.msteamsAppClient.(*mocks.Client).On("RefreshSubscriptionsPeriodically", mock.Anything, mock.Anything).Return(nil)
	plugin.msteamsAppClient.(*mocks.Client).On("SubscribeToChannels", mock.Anything, plugin.configuration.WebhookSecret).Return("channel-subscription-id", nil)
	plugin.msteamsAppClient.(*mocks.Client).On("SubscribeToChats", mock.Anything, plugin.configuration.WebhookSecret).Return("chats-subscription-id", nil)
	plugin.msteamsAppClient.(*mocks.Client).On("SubscribeToChannel", mock.Anything, mock.Anything, "/plugins/com.mattermost.msteams-sync/", plugin.configuration.WebhookSecret).Return(&clientmodels.Subscription{ID: "channel-subscription-id"}, nil)
	plugin.msteamsAppClient.(*mocks.Client).Test(t)
	bot := &model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: "Created by the MS Teams Sync plugin.",
	}
	config := model.Config{}
	config.SetDefaults()
	plugin.API.(*plugintest.API).On("KVGet", "cron_monitoring_system").Return(nil, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVGet", lastReceivedChangeKey).Return([]byte{}, nil).Times(1)
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
	plugin.metricsService.(*metricsmocks.Metrics).On("IncrementHTTPRequests")
	plugin.metricsService.(*metricsmocks.Metrics).On("ObserveAPIEndpointDuration", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("float64"))

	plugin.API.(*plugintest.API).Test(t)
	_ = plugin.OnActivate()
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

func TestMessageHasBeenPostedNewMessage(t *testing.T) {
	plugin := newTestPlugin(t)

	channel := model.Channel{
		Id:     "channel-id",
		TeamId: "team-id",
	}
	post := model.Post{
		Id:        "post-id",
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
		UserId:    "user-id",
		ChannelId: channel.Id,
		Message:   "message",
	}

	link := storemodels.ChannelLink{
		MattermostTeamID:    "team-id",
		MattermostChannelID: "channel-id",
		MSTeamsTeam:         "ms-team-id",
		MSTeamsChannel:      "ms-channel-id",
	}
	plugin.store.(*storemocks.Store).On("GetLinkByChannelID", "channel-id").Return(&link, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetChannel", "channel-id").Return(&channel, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetUser", "user-id").Return(&model.User{Id: "user-id", Username: "test-user"}, nil).Times(1)
	plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "user-id").Return(&oauth2.Token{}, nil).Times(1)
	now := time.Now()
	plugin.store.(*storemocks.Store).On("LinkPosts", storemodels.PostInfo{
		MattermostID:        "post-id",
		MSTeamsID:           "new-message-id",
		MSTeamsChannel:      "ms-channel-id",
		MSTeamsLastUpdateAt: now,
	}, (*sql.Tx)(nil)).Return(nil).Times(1)
	clientMock := plugin.clientBuilderWithToken("", "", "", "", nil, nil, nil)
	clientMock.(*mocks.Client).On("SendMessageWithAttachments", "ms-team-id", "ms-channel-id", "", "<p>message</p>\n", []*clientmodels.Attachment(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{ID: "new-message-id", LastUpdateAt: now}, nil)
	plugin.metricsService.(*metricsmocks.Metrics).On("ObserveMessagesCount", actionCreated, actionSourceMattermost, isNotDirectMessage).Times(1)
	plugin.metricsService.(*metricsmocks.Metrics).On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", mock.AnythingOfType("float64")).Once()

	plugin.MessageHasBeenPosted(nil, &post)
}

func TestMessageHasBeenPostedNewMessageWithoutChannelLink(t *testing.T) {
	plugin := newTestPlugin(t)

	channel := model.Channel{
		Id:     "channel-id",
		TeamId: "team-id",
	}
	post := model.Post{
		Id:        "post-id",
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
		UserId:    "user-id",
		ChannelId: channel.Id,
		Message:   "message",
	}

	plugin.API.(*plugintest.API).On("GetChannel", "channel-id").Return(&channel, nil).Times(1)
	plugin.store.(*storemocks.Store).On("GetLinkByChannelID", "channel-id").Return(nil, model.NewAppError("test", "not-found", nil, "", http.StatusNotFound)).Times(1)
	plugin.MessageHasBeenPosted(nil, &post)
}

func TestMessageHasBeenPostedNewMessageWithFailureSending(t *testing.T) {
	plugin := newTestPlugin(t)

	channel := model.Channel{
		Id:     "channel-id",
		TeamId: "team-id",
	}
	post := model.Post{
		Id:        "post-id",
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
		UserId:    "user-id",
		ChannelId: channel.Id,
		Message:   "message",
	}

	link := storemodels.ChannelLink{
		MattermostTeamID:    "team-id",
		MattermostChannelID: "channel-id",
		MSTeamsTeam:         "ms-team-id",
		MSTeamsChannel:      "ms-channel-id",
	}
	plugin.store.(*storemocks.Store).On("GetLinkByChannelID", "channel-id").Return(&link, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetChannel", "channel-id").Return(&channel, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetUser", "user-id").Return(&model.User{Id: "user-id", Username: "test-user"}, nil).Times(1)
	plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "user-id").Return(&oauth2.Token{}, nil).Times(1)
	clientMock := plugin.clientBuilderWithToken("", "", "", "", nil, nil, nil)
	clientMock.(*mocks.Client).On("SendMessageWithAttachments", "ms-team-id", "ms-channel-id", "", "<p>message</p>\n", []*clientmodels.Attachment(nil), []models.ChatMessageMentionable{}).Return(nil, errors.New("Unable to send the message"))
	plugin.API.(*plugintest.API).On("LogError", "Error creating post on MS Teams", "error", "Unable to send the message").Return(nil)
	plugin.API.(*plugintest.API).On("LogWarn", "Unable to handle message sent", "error", "Unable to send the message").Return(nil)
	plugin.metricsService.(*metricsmocks.Metrics).On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "false", mock.AnythingOfType("float64")).Once()

	plugin.MessageHasBeenPosted(nil, &post)
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
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
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
				store.On("GetTokenForMSTeamsUser", testutils.GetTeamsUserID()).Return(nil, nil).Times(1)
			},
			ExpectedError: "not connected user",
		},
		{
			Name: "GetClientForTeamsUser: Valid",
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMSTeamsUser", testutils.GetTeamsUserID()).Return(&oauth2.Token{}, nil).Times(1)
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
				metrics.On("ObserveUpstreamUsersTotal", int64(1)).Times(1)
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
				metrics.On("ObserveUpstreamUsersTotal", int64(1)).Times(1)
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
				metrics.On("ObserveUpstreamUsersTotal", int64(1)).Times(1)
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

func TestConnectTeamsAppClient(t *testing.T) {
	for _, test := range []struct {
		Name          string
		SetupAPI      func(*plugintest.API)
		SetupClient   func(*mocks.Client)
		ExpectedError string
	}{
		{
			Name: "ConnectTeamsAppClient: Unable to connect to the app client",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to connect to the app client", "error", mock.Anything).Times(1)
			},
			SetupClient: func(client *mocks.Client) {
				client.On("Connect").Return(errors.New("unable to connect to the app client")).Times(1)
			},
			ExpectedError: "unable to connect to the app client",
		},
		{
			Name:     "ConnectTeamsAppClient: Valid",
			SetupAPI: func(api *plugintest.API) {},
			SetupClient: func(client *mocks.Client) {
				client.On("Connect").Return(nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupClient(p.msteamsAppClient.(*mocks.Client))
			err := p.connectTeamsAppClient()
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestStart(t *testing.T) {
	mockSiteURL := "mockSiteURL"
	for _, test := range []struct {
		Name        string
		SetupAPI    func(*plugintest.API)
		SetupClient func(*mocks.Client)
		SetupStore  func(*storemocks.Store)
	}{
		{
			Name: "Start: Unable to connect to the app client",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to connect to the app client", "error", mock.Anything).Times(1)
				api.On("LogError", "Unable to connect to the msteams", "error", mock.Anything).Times(1)
			},
			SetupClient: func(client *mocks.Client) {
				client.On("Connect").Return(errors.New("unable to connect to the app client")).Times(1)
			},
			SetupStore: func(s *storemocks.Store) {},
		},
		{
			Name: "Start: Valid",
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
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			p.metricsService.(*metricsmocks.Metrics).On("ObserveChangeEventQueueCapacity", int64(5000)).Times(1)
			mutex, _ := cluster.NewMutex(p.API, clusterMutexKey)
			p.clusterMutex = mutex
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupClient(p.msteamsAppClient.(*mocks.Client))
			test.SetupStore(p.store.(*storemocks.Store))
			p.start(nil)
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
