package main

import (
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-api/cluster"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"
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
		clientBuilderWithToken: func(redirectURL, tenantID, clientId, clientSecret string, token *oauth2.Token, logError func(string, ...any)) msteams.Client {
			return clientMock
		},
	}
	plugin.store.(*storemocks.Store).Test(t)

	plugin.msteamsAppClient.(*mocks.Client).On("ClearSubscriptions").Return(nil)
	plugin.msteamsAppClient.(*mocks.Client).On("RefreshSubscriptionsPeriodically", mock.Anything, mock.Anything).Return(nil)
	plugin.msteamsAppClient.(*mocks.Client).On("SubscribeToChannels", mock.Anything, plugin.configuration.WebhookSecret).Return("channel-subscription-id", nil)
	plugin.msteamsAppClient.(*mocks.Client).On("SubscribeToChats", mock.Anything, plugin.configuration.WebhookSecret).Return("chats-subscription-id", nil)
	plugin.msteamsAppClient.(*mocks.Client).Test(t)
	bot := &model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: "Created by the MS Teams Sync plugin.",
	}
	config := model.Config{}
	config.SetDefaults()
	plugin.API.(*plugintest.API).On("KVGet", lastReceivedChangeKey).Return([]byte{}, nil)
	plugin.API.(*plugintest.API).On("GetServerVersion").Return("7.8.0")
	plugin.API.(*plugintest.API).On("GetBundlePath").Return("./dist", nil)
	plugin.API.(*plugintest.API).On("Conn", true).Return("connection-id", nil)
	plugin.API.(*plugintest.API).On("GetUnsanitizedConfig").Return(&config)
	plugin.API.(*plugintest.API).On("EnsureBotUser", bot).Return("bot-user-id", nil).Times(1)
	plugin.API.(*plugintest.API).On("RegisterCommand", mock.Anything).Return(nil).Times(1)
	plugin.API.(*plugintest.API).On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	plugin.API.(*plugintest.API).On("KVList", 0, 1000000000).Return([]string{}, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_subscriptions_cluster_mutex", []byte{0x1}, model.PluginKVSetOptions{Atomic: true, ExpireInSeconds: 15}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_subscriptions_cluster_mutex", []byte(nil), model.PluginKVSetOptions{Atomic: false, ExpireInSeconds: 0}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_mmi_bot_ensure", []byte{0x1}, model.PluginKVSetOptions{Atomic: true, ExpireInSeconds: 15}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_mmi_bot_ensure", []byte(nil), model.PluginKVSetOptions{Atomic: false, ExpireInSeconds: 0}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(1)
	plugin.API.(*plugintest.API).Test(t)

	_ = plugin.OnActivate()
	plugin.userID = "bot-user-id"
	return plugin
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
	}).Return(nil).Times(1)
	clientMock := plugin.clientBuilderWithToken("", "", "", "", nil, nil)
	clientMock.(*mocks.Client).On("SendMessageWithAttachments", "ms-team-id", "ms-channel-id", "", "message", []*msteams.Attachment(nil)).Return(&msteams.Message{ID: "new-message-id", LastUpdateAt: now}, nil)

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
	clientMock := plugin.clientBuilderWithToken("", "", "", "", nil, nil)
	clientMock.(*mocks.Client).On("SendMessageWithAttachments", "ms-team-id", "ms-channel-id", "", "message", []*msteams.Attachment(nil)).Return(nil, errors.New("Unable to send the message"))
	plugin.API.(*plugintest.API).On("LogWarn", "Error creating post", "error", "Unable to send the message").Return(nil)
	plugin.API.(*plugintest.API).On("LogError", "Unable to handle message sent", "error", "Unable to send the message").Return(nil)

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
				store.On("GetTokenForMSTeamsUser", testutils.GetTeamUserID()).Return(nil, nil).Times(1)
			},
			ExpectedError: "not connected user",
		},
		{
			Name: "GetClientForTeamsUser: Valid",
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMSTeamsUser", testutils.GetTeamUserID()).Return(&oauth2.Token{}, nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupStore(p.store.(*storemocks.Store))
			resp, err := p.GetClientForTeamsUser(testutils.GetTeamUserID())
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
		Name        string
		SetupAPI    func(*plugintest.API)
		SetupStore  func(*storemocks.Store)
		SetupClient func(*mocks.Client)
	}{
		{
			Name: "SyncUsers: Unable to get the user list",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to sync users", "error", mock.Anything).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {},
			SetupClient: func(client *mocks.Client) {
				client.On("ListUsers").Return(nil, errors.New("unable to get the user list")).Times(1)
			},
		},
		{
			Name: "SyncUsers: Unable to get the users",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to sync users", "error", mock.Anything).Times(1)
				api.On("GetUsers", &model.UserGetOptions{
					Active:  true,
					Page:    0,
					PerPage: math.MaxInt32,
				}).Return(nil, testutils.GetInternalServerAppError("unable to get the users")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {},
			SetupClient: func(client *mocks.Client) {
				client.On("ListUsers").Return([]msteams.User{
					{
						ID:          testutils.GetTeamUserID(),
						DisplayName: "mockDisplayName",
					},
				}, nil).Times(1)
			},
		},
		{
			Name: "SyncUsers: Unable to create the user",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to sync user", "error", mock.Anything).Times(1)
				api.On("GetUsers", &model.UserGetOptions{
					Active:  true,
					Page:    0,
					PerPage: math.MaxInt32,
				}).Return([]*model.User{
					testutils.GetUser(model.SystemAdminRoleId, "test@test.com"),
				}, nil).Times(1)
				api.On("CreateUser", mock.AnythingOfType("*model.User")).Return(nil, testutils.GetInternalServerAppError("unable to create the user")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {},
			SetupClient: func(client *mocks.Client) {
				client.On("ListUsers").Return([]msteams.User{
					{
						ID:          testutils.GetTeamUserID(),
						DisplayName: "mockDisplayName",
					},
				}, nil).Times(1)
			},
		},
		{
			Name: "SyncUsers: Unable to store the user info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to sync user", "error", mock.Anything).Times(1)
				api.On("GetUsers", &model.UserGetOptions{
					Active:  true,
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
				store.On("SetUserInfo", testutils.GetID(), testutils.GetTeamUserID(), mock.AnythingOfType("*oauth2.Token")).Return(testutils.GetInternalServerAppError("unable to store the user info")).Times(1)
			},
			SetupClient: func(client *mocks.Client) {
				client.On("ListUsers").Return([]msteams.User{
					{
						ID:          testutils.GetTeamUserID(),
						DisplayName: "mockDisplayName",
					},
				}, nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*mocks.Client))
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
		},
		{
			Name: "Start: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &mockSiteURL,
					},
				})
			},
			SetupClient: func(client *mocks.Client) {
				client.On("Connect").Return(nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			mutex, _ := cluster.NewMutex(p.API, clusterMutexKey)
			p.clusterMutex = mutex
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupClient(p.msteamsAppClient.(*mocks.Client))
			p.start(nil)
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
