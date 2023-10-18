package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gosimple/slug"
	mocksPlugin "github.com/mattermost/mattermost-plugin-msteams-sync/server/handlers/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	mocksClient "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	storemocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"
)

type pluginMock struct {
	api                        plugin.API
	store                      store.Store
	syncDirectMessages         bool
	syncGuestUsers             bool
	maxSizeForCompleteDownload int
	bufferSizeForStreaming     int
	botUserID                  string
	url                        string
	appClient                  msteams.Client
	userClient                 msteams.Client
	teamsUserClient            msteams.Client
	metrics                    metrics.Metrics
}

func (pm *pluginMock) GetAPI() plugin.API                              { return pm.api }
func (pm *pluginMock) GetStore() store.Store                           { return pm.store }
func (pm *pluginMock) GetSyncDirectMessages() bool                     { return pm.syncDirectMessages }
func (pm *pluginMock) GetSyncGuestUsers() bool                         { return pm.syncGuestUsers }
func (pm *pluginMock) GetMaxSizeForCompleteDownload() int              { return pm.maxSizeForCompleteDownload }
func (pm *pluginMock) GetBufferSizeForStreaming() int                  { return pm.bufferSizeForStreaming }
func (pm *pluginMock) GetBotUserID() string                            { return pm.botUserID }
func (pm *pluginMock) GetURL() string                                  { return pm.url }
func (pm *pluginMock) GetMetrics() metrics.Metrics                     { return pm.metrics }
func (pm *pluginMock) GetClientForApp() msteams.Client                 { return pm.appClient }
func (pm *pluginMock) GetClientForUser(string) (msteams.Client, error) { return pm.userClient, nil }
func (pm *pluginMock) GetClientForTeamsUser(string) (msteams.Client, error) {
	return pm.teamsUserClient, nil
}
func (pm *pluginMock) GenerateRandomPassword() string {
	return ""
}

func newTestHandler() *ActivityHandler {
	return New(&pluginMock{
		appClient:                  &mocksClient.Client{},
		userClient:                 &mocksClient.Client{},
		teamsUserClient:            &mocksClient.Client{},
		store:                      &storemocks.Store{},
		api:                        &plugintest.API{},
		botUserID:                  "bot-user-id",
		url:                        "fake-url",
		syncDirectMessages:         false,
		syncGuestUsers:             false,
		maxSizeForCompleteDownload: 20,
		bufferSizeForStreaming:     20,
	})
}

func TestGetOrCreateSyntheticUser(t *testing.T) {
	for _, test := range []struct {
		Name           string
		TryCreate      bool
		SetupStore     func(*storemocks.Store)
		SetupAPI       func(*plugintest.API)
		ExpectedResult string
		ExpectedError  bool
	}{
		{
			Name: "Unknown user but matching an already existing synthetic user",
			SetupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return("", nil).Times(1)
				store.On("SetUserInfo", "new-user-id", testutils.GetTeamsUserID(), mock.Anything).Return(nil).Times(1)
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByEmail", testutils.GetTestEmail()).Return(&model.User{Id: "new-user-id"}, nil).Times(1)
			},
			ExpectedResult: "new-user-id",
		},
		{
			Name:      "Unknown user not matching an already existing synthetic user",
			TryCreate: true,
			SetupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return("", nil).Times(1)
				store.On("SetUserInfo", "new-user-id", testutils.GetTeamsUserID(), mock.Anything).Return(nil).Times(1)
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByEmail", testutils.GetTestEmail()).Return(nil, model.NewAppError("test", "not-found", nil, "", http.StatusNotFound)).Times(1)
				api.On("CreateUser", mock.MatchedBy(func(user *model.User) bool {
					if user.Username != "msteams_"+slug.Make("Unknown User") {
						return false
					}
					if user.Email != testutils.GetTestEmail() {
						return false
					}
					return true
				})).Return(&model.User{Id: "new-user-id"}, nil).Times(1)
				api.On("UpdatePreferencesForUser", "new-user-id", mock.MatchedBy(func(preferences model.Preferences) bool {
					if len(preferences) == 0 || len(preferences) > 1 {
						return false
					}
					if preferences[0].UserId != "new-user-id" {
						return false
					}
					return true
				})).Return(model.NewAppError("test", "something went wrong", nil, "", http.StatusInternalServerError)).Once()
				api.On("LogError", "Unable to disable email notifications for new user", "mmuserID", "new-user-id", "error", "test: something went wrong")
			},
			ExpectedResult: "new-user-id",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			ah := newTestHandler()
			test.SetupAPI(ah.plugin.GetAPI().(*plugintest.API))
			test.SetupStore(ah.plugin.GetStore().(*storemocks.Store))
			result, err := ah.getOrCreateSyntheticUser(&msteams.User{
				ID:          testutils.GetTeamsUserID(),
				Mail:        testutils.GetTestEmail(),
				DisplayName: "Unknown User",
			}, test.TryCreate)
			assert.Equal(test.ExpectedResult, result)
			if test.ExpectedError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestGetChatChannelID(t *testing.T) {
	ah := ActivityHandler{}
	client := mocksClient.NewClient(t)
	mockAPI := &plugintest.API{}
	store := storemocks.NewStore(t)

	for _, testCase := range []struct {
		description      string
		chat             *msteams.Chat
		messageID        string
		expectedResponse string
		expectedError    string
		setupPlugin      func(plugin *mocksPlugin.PluginIface)
		setupAPI         func()
		setupStore       func()
		setupClient      func(client *mocksClient.Client)
	}{
		{
			description: "Successfully got the ID of direct channel",
			chat: &msteams.Chat{
				Members: []msteams.ChatMember{
					{UserID: testutils.GetUserID() + "1"},
					{UserID: testutils.GetUserID() + "2"},
				},
				Type: "D",
			},
			messageID: testutils.GetMessageID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForApp").Return(client).Times(2)
				p.On("GetStore").Return(store).Times(2)
				p.On("GetAPI").Return(mockAPI).Once()
			},
			setupAPI: func() {
				mockAPI.On("GetDirectChannel", "mock-mmUserID1", "mock-mmUserID2").Return(&model.Channel{
					Id: testutils.GetChannelID(),
				}, nil)
			},
			setupStore: func() {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"1").Return("mock-mmUserID1", nil).Once()
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"2").Return("mock-mmUserID2", nil).Once()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetUser", testutils.GetUserID()+"1").Return(&msteams.User{ID: testutils.GetUserID() + "1"}, nil).Once()
				client.On("GetUser", testutils.GetUserID()+"2").Return(&msteams.User{ID: testutils.GetUserID() + "2"}, nil).Once()
			},
			expectedResponse: testutils.GetChannelID(),
		},
		{
			description: "Successfully got the ID of group channel",
			chat: &msteams.Chat{
				Members: []msteams.ChatMember{
					{UserID: testutils.GetUserID() + "1"},
					{UserID: testutils.GetUserID() + "2"},
				},
				Type: "G",
			},
			messageID: testutils.GetMessageID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForApp").Return(client).Times(2)
				p.On("GetStore").Return(store).Times(2)
				p.On("GetAPI").Return(mockAPI).Once()
			},
			setupAPI: func() {
				mockAPI.On("GetGroupChannel", []string{"mock-mmUserID1", "mock-mmUserID2"}).Return(&model.Channel{
					Id: testutils.GetChannelID(),
				}, nil)
			},
			setupStore: func() {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"1").Return("mock-mmUserID1", nil)
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"2").Return("mock-mmUserID2", nil)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetUser", testutils.GetUserID()+"1").Return(&msteams.User{ID: testutils.GetUserID() + "1"}, nil).Once()
				client.On("GetUser", testutils.GetUserID()+"2").Return(&msteams.User{ID: testutils.GetUserID() + "2"}, nil).Once()
			},
			expectedResponse: testutils.GetChannelID(),
		},
		{
			description: "Unable to get or create synthetic user",
			chat: &msteams.Chat{
				Members: []msteams.ChatMember{
					{UserID: "mock-userID"},
				},
			},
			messageID: testutils.GetMessageID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForApp").Return(client).Once()
				p.On("GetStore").Return(store).Times(2)
				p.On("GetAPI").Return(mockAPI).Times(2)
			},
			setupAPI: func() {
				mockAPI.On("GetUserByEmail", "mock-userID@msteamssync").Return(&model.User{
					Id: "mock-Id",
				}, nil)
			},
			setupStore: func() {
				var token *oauth2.Token
				store.On("TeamsToMattermostUserID", "mock-userID").Return("", errors.New("Error while getting mattermost userID"))
				store.On("SetUserInfo", "mock-Id", "mock-userID", token).Return(errors.New("Error while setting user info"))
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetUser", "mock-userID").Return(&msteams.User{
					DisplayName: "New display name",
					ID:          "mock-userID",
					Mail:        "mock-userID@msteamssync",
				}, nil)
			},
			expectedError:    "Error while setting user info",
			expectedResponse: "",
		},
		{
			description: "Not enough user for creating a channel",
			chat: &msteams.Chat{
				Members: []msteams.ChatMember{
					{UserID: testutils.GetUserID()},
				},
			},
			messageID: testutils.GetMessageID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForApp").Return(client).Once()
				p.On("GetStore").Return(store).Once()
			},
			setupAPI: func() {},
			setupStore: func() {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()).Return("mock-mmUserID", nil)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetUser", testutils.GetUserID()).Return(&msteams.User{ID: testutils.GetUserID()}, nil).Once()
			},
			expectedError: "not enough users for creating a channel",
		},
		{
			description: "Direct or group channel not found",
			chat: &msteams.Chat{
				Members: []msteams.ChatMember{
					{UserID: testutils.GetUserID() + "1"},
					{UserID: testutils.GetUserID() + "2"},
				},
			},
			messageID: testutils.GetMessageID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForApp").Return(client).Times(2)
				p.On("GetStore").Return(store).Times(2)
			},
			setupAPI: func() {},
			setupStore: func() {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"1").Return("mock-mmUserID1", nil)
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"2").Return("mock-mmUserID2", nil)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetUser", testutils.GetUserID()+"1").Return(&msteams.User{ID: testutils.GetUserID() + "1"}, nil).Once()
				client.On("GetUser", testutils.GetUserID()+"2").Return(&msteams.User{ID: testutils.GetUserID() + "2"}, nil).Once()
			},
			expectedError: "dm/gm not found",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			mockAPI.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			testCase.setupStore()
			testCase.setupAPI()
			testCase.setupClient(client)
			ah.plugin = p

			resp, err := ah.getChatChannelID(testCase.chat)
			assert.Equal(t, resp, testCase.expectedResponse)
			if testCase.expectedError != "" {
				assert.EqualError(t, err, testCase.expectedError)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestGetMessageFromChannel(t *testing.T) {
	ah := ActivityHandler{}
	mockAPI := &plugintest.API{}
	client := mocksClient.NewClient(t)

	for _, testCase := range []struct {
		description   string
		userID        string
		teamID        string
		channelID     string
		messageID     string
		expectedError string
		setupPlugin   func(p *mocksPlugin.PluginIface)
		setupClient   func()
	}{
		{
			description: "Successfully got message from channel",
			userID:      testutils.GetUserID(),
			teamID:      testutils.GetTeamsUserID(),
			channelID:   testutils.GetChannelID(),
			messageID:   testutils.GetMessageID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForApp").Return(client)
			},
			setupClient: func() {
				client.On("GetMessage", testutils.GetTeamsUserID(), testutils.GetChannelID(), testutils.GetMessageID()).Return(&msteams.Message{}, nil)
			},
		},
		{
			description: "Unable to get original post",
			userID:      testutils.GetUserID(),
			teamID:      testutils.GetTeamsUserID(),
			channelID:   testutils.GetChannelID(),
			messageID:   "mock-messageID",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForApp").Return(client)
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func() {
				client.On("GetMessage", testutils.GetTeamsUserID(), testutils.GetChannelID(), "mock-messageID").Return(nil, errors.New("Error while getting the original post"))
			},
			expectedError: "Error while getting the original post",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			mockAPI.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			testCase.setupClient()
			ah.plugin = p

			resp, err := ah.getMessageFromChannel(testCase.teamID, testCase.channelID, testCase.messageID)
			if testCase.expectedError != "" {
				assert.Nil(t, resp)
				assert.EqualError(t, err, testCase.expectedError)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestGetReplyFromChannel(t *testing.T) {
	ah := ActivityHandler{}
	mockAPI := &plugintest.API{}
	client := mocksClient.NewClient(t)

	for _, testCase := range []struct {
		description   string
		userID        string
		teamID        string
		channelID     string
		messageID     string
		replyID       string
		expectedError string
		setupPlugin   func(plugin *mocksPlugin.PluginIface)
		setupClient   func()
	}{
		{
			description: "Successfully got reply from channel",
			userID:      testutils.GetUserID(),
			teamID:      testutils.GetTeamsUserID(),
			channelID:   testutils.GetChannelID(),
			messageID:   testutils.GetMessageID(),
			replyID:     testutils.GetReplyID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForApp").Return(client)
			},
			setupClient: func() {
				client.On("GetReply", testutils.GetTeamsUserID(), testutils.GetChannelID(), testutils.GetMessageID(), testutils.GetReplyID()).Return(&msteams.Message{}, nil)
			},
		},
		{
			description: "Unable to get original post",
			userID:      testutils.GetUserID(),
			teamID:      testutils.GetTeamsUserID(),
			channelID:   testutils.GetChannelID(),
			messageID:   testutils.GetMessageID(),
			replyID:     "mock-replyID",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForApp").Return(client)
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func() {
				client.On("GetReply", testutils.GetTeamsUserID(), testutils.GetChannelID(), testutils.GetMessageID(), "mock-replyID").Return(nil, errors.New("Error while getting the original post"))
			},
			expectedError: "Error while getting the original post",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			mockAPI.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			testCase.setupClient()
			ah.plugin = p

			resp, err := ah.getReplyFromChannel(testCase.teamID, testCase.channelID, testCase.messageID, testCase.replyID)
			if testCase.expectedError != "" {
				assert.Nil(t, resp)
				assert.EqualError(t, err, testCase.expectedError)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestGetUserIDForChannelLink(t *testing.T) {
	ah := ActivityHandler{}
	store := storemocks.NewStore(t)

	for _, testCase := range []struct {
		description      string
		channelID        string
		teamID           string
		expectedResponse string
		setupPlugin      func(plugin *mocksPlugin.PluginIface)
	}{
		{
			description:      "Get bot user ID",
			teamID:           testutils.GetTeamsUserID(),
			expectedResponse: testutils.GetUserID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
				p.On("GetBotUserID").Return(testutils.GetUserID())
				store.On("GetLinkByMSTeamsChannelID", testutils.GetTeamsUserID(), "").Return(nil, errors.New("Error while getting channel link"))
			},
		},
		{
			description:      "Get creator of channel link",
			teamID:           testutils.GetTeamsUserID(),
			channelID:        testutils.GetChannelID(),
			expectedResponse: "mock-creator",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
				store.On("GetLinkByMSTeamsChannelID", testutils.GetTeamsUserID(), testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					Creator: "mock-creator",
				}, nil)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			ah.plugin = p

			resp := ah.getUserIDForChannelLink(testCase.teamID, testCase.channelID)
			assert.Equal(t, resp, testCase.expectedResponse)
		})
	}
}

func TestGetMessageAndChatFromActivityIds(t *testing.T) {
	ah := ActivityHandler{}

	for _, testCase := range []struct {
		description   string
		activityIds   msteams.ActivityIds
		setupPlugin   func(plugin *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *storemocks.Store)
		setupClient   func(client *mocksClient.Client)
		setupStore    func(store *storemocks.Store)
		setupAPI      func(api *plugintest.API)
		expectedError string
	}{
		{
			description: "Successfully get message and chat from activity ID",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *storemocks.Store) {
				p.On("GetClientForApp").Return(client)
				p.On("GetClientForTeamsUser", testutils.GetUserID()).Return(client, nil)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetUserID(),
						},
					},
					ID: testutils.GetChatID(),
				}, nil)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{}, nil)
			},
			setupStore: func(store *storemocks.Store) {},
			setupAPI:   func(api *plugintest.API) {},
		},
		{
			description: "Unable to get original chat",
			activityIds: msteams.ActivityIds{
				ChatID: "mock-ChatID",
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *storemocks.Store) {
				p.On("GetClientForApp").Return(client).Once()
				p.On("GetAPI").Return(mockAPI).Once()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", "mock-ChatID").Return(nil, errors.New("Error while getting original chat"))
			},
			setupStore: func(store *storemocks.Store) {},
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get original chat", "chatID", "mock-ChatID", "error", errors.New("Error while getting original chat")).Return().Once()
			},
			expectedError: "Error while getting original chat",
		},
		{
			description: "Unable to get message from chat",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: "mock-MessageID",
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *storemocks.Store) {
				p.On("GetClientForApp").Return(client).Once()
				p.On("GetClientForTeamsUser", testutils.GetUserID()).Return(client, nil).Once()
				p.On("GetAPI").Return(mockAPI).Once()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetUserID(),
						},
					},
					ID: testutils.GetChatID(),
				}, nil).Once()
				client.On("GetChatMessage", testutils.GetChatID(), "mock-MessageID").Return(nil, errors.New("Error while getting chat message")).Once()
			},
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get message from chat", "chatID", testutils.GetChatID(), "messageID", "mock-MessageID", "error", errors.New("Error while getting chat message")).Once()
			},
			setupStore:    func(store *storemocks.Store) {},
			expectedError: "Error while getting chat message",
		},
		{
			description: "Unable to get reply from channel",
			activityIds: msteams.ActivityIds{
				ReplyID:   testutils.GetReplyID(),
				MessageID: testutils.GetMessageID(),
				TeamID:    testutils.GetTeamsUserID(),
				ChannelID: testutils.GetChannelID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *storemocks.Store) {
				p.On("GetClientForApp").Return(client)
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetReply", testutils.GetTeamsUserID(), testutils.GetChannelID(), testutils.GetMessageID(), testutils.GetReplyID()).Return(nil, errors.New("Error while getting reply from channel"))
			},
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get reply from channel", "replyID", testutils.GetReplyID(), "error", errors.New("Error while getting reply from channel"))
			},
			setupStore:    func(store *storemocks.Store) {},
			expectedError: "Error while getting reply from channel",
		},
		{
			description: "Unable to get message from channel",
			activityIds: msteams.ActivityIds{
				MessageID: "mock-MessageID",
				TeamID:    testutils.GetTeamsUserID(),
				ChannelID: testutils.GetChannelID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *storemocks.Store) {
				p.On("GetClientForApp").Return(client)
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetMessage", testutils.GetTeamsUserID(), testutils.GetChannelID(), "mock-MessageID").Return(nil, errors.New("Error while getting message from channel"))
			},
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get message from channel", "messageID", "mock-MessageID", "error", errors.New("Error while getting message from channel"))
			},
			setupStore:    func(store *storemocks.Store) {},
			expectedError: "Error while getting message from channel",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			client := mocksClient.NewClient(t)
			mockAPI := &plugintest.API{}
			store := storemocks.NewStore(t)

			testCase.setupPlugin(p, mockAPI, client, store)
			testCase.setupClient(client)
			testCase.setupStore(store)
			testCase.setupAPI(mockAPI)
			ah.plugin = p

			message, chat, err := ah.getMessageAndChatFromActivityIds(testCase.activityIds)
			if testCase.expectedError != "" {
				assert.Nil(t, message)
				assert.Nil(t, chat)
				assert.EqualError(t, err, testCase.expectedError)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
