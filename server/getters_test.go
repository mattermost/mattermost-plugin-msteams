package main

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gosimple/slug"
	metricsmocks "github.com/mattermost/mattermost-plugin-msteams/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"
)

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
					if !user.EmailVerified {
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
			},
			ExpectedResult: "new-user-id",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			ah := ActivityHandler{plugin: p}

			assert := assert.New(t)
			test.SetupAPI(ah.plugin.GetAPI().(*plugintest.API))
			testutils.MockLogs(ah.plugin.GetAPI().(*plugintest.API))
			test.SetupStore(ah.plugin.GetStore().(*storemocks.Store))
			result, err := ah.getOrCreateSyntheticUser(&clientmodels.User{
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
	for _, testCase := range []struct {
		description      string
		chat             *clientmodels.Chat
		messageID        string
		expectedResponse string
		expectedError    string
		expectedUserIDs  []string
		setupAPI         func(api *plugintest.API)
		setupStore       func(store *storemocks.Store)
		setupClient      func(client *clientmocks.Client)
	}{
		{
			description: "Successfully got the ID of direct channel",
			chat: &clientmodels.Chat{
				Members: []clientmodels.ChatMember{
					{UserID: testutils.GetUserID() + "1"},
					{UserID: testutils.GetUserID() + "2"},
				},
				Type: "D",
			},
			messageID: testutils.GetMessageID(),
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetDirectChannel", "mock-mmUserID1", "mock-mmUserID2").Return(&model.Channel{
					Id: testutils.GetChannelID(),
				}, nil)
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"1").Return("mock-mmUserID1", nil).Once()
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"2").Return("mock-mmUserID2", nil).Once()
			},
			expectedUserIDs: []string{"mock-mmUserID1", "mock-mmUserID2"},
			setupClient: func(client *clientmocks.Client) {
				client.On("GetUser", testutils.GetUserID()+"1").Return(&clientmodels.User{ID: testutils.GetUserID() + "1"}, nil).Once()
				client.On("GetUser", testutils.GetUserID()+"2").Return(&clientmodels.User{ID: testutils.GetUserID() + "2"}, nil).Once()
			},
			expectedResponse: testutils.GetChannelID(),
		},
		{
			description: "Successfully got the ID of group channel",
			chat: &clientmodels.Chat{
				Members: []clientmodels.ChatMember{
					{UserID: testutils.GetUserID() + "1"},
					{UserID: testutils.GetUserID() + "2"},
				},
				Type: "G",
			},
			messageID: testutils.GetMessageID(),
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetGroupChannel", []string{"mock-mmUserID1", "mock-mmUserID2"}).Return(&model.Channel{
					Id: testutils.GetChannelID(),
				}, nil)
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"1").Return("mock-mmUserID1", nil)
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"2").Return("mock-mmUserID2", nil)
			},
			expectedUserIDs: []string{"mock-mmUserID1", "mock-mmUserID2"},
			setupClient: func(client *clientmocks.Client) {
				client.On("GetUser", testutils.GetUserID()+"1").Return(&clientmodels.User{ID: testutils.GetUserID() + "1"}, nil).Once()
				client.On("GetUser", testutils.GetUserID()+"2").Return(&clientmodels.User{ID: testutils.GetUserID() + "2"}, nil).Once()
			},
			expectedResponse: testutils.GetChannelID(),
		},
		{
			description: "Unable to get or create synthetic user",
			chat: &clientmodels.Chat{
				Members: []clientmodels.ChatMember{
					{UserID: "mock-userID"},
				},
			},
			messageID: testutils.GetMessageID(),
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetUserByEmail", "mock-userID@msteamssync").Return(&model.User{
					Id: "mock-Id",
				}, nil)
			},
			setupStore: func(store *storemocks.Store) {
				var token *oauth2.Token
				store.On("TeamsToMattermostUserID", "mock-userID").Return("", errors.New("Error while getting mattermost userID"))
				store.On("SetUserInfo", "mock-Id", "mock-userID", token).Return(errors.New("Error while setting user info"))
			},
			setupClient: func(client *clientmocks.Client) {
				client.On("GetUser", "mock-userID").Return(&clientmodels.User{
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
			chat: &clientmodels.Chat{
				Members: []clientmodels.ChatMember{
					{UserID: testutils.GetUserID()},
				},
			},
			messageID: testutils.GetMessageID(),
			setupAPI:  func(mockAPI *plugintest.API) {},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()).Return("mock-mmUserID", nil)
			},
			expectedUserIDs: nil,
			setupClient: func(client *clientmocks.Client) {
				client.On("GetUser", testutils.GetUserID()).Return(&clientmodels.User{ID: testutils.GetUserID()}, nil).Once()
			},
			expectedError: "not enough users for creating a channel",
		},
		{
			description: "Direct or group channel not found",
			chat: &clientmodels.Chat{
				Members: []clientmodels.ChatMember{
					{UserID: testutils.GetUserID() + "1"},
					{UserID: testutils.GetUserID() + "2"},
				},
			},
			messageID: testutils.GetMessageID(),
			setupAPI:  func(mockAPI *plugintest.API) {},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"1").Return("mock-mmUserID1", nil)
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"2").Return("mock-mmUserID2", nil)
			},
			expectedUserIDs: nil,
			setupClient: func(client *clientmocks.Client) {
				client.On("GetUser", testutils.GetUserID()+"1").Return(&clientmodels.User{ID: testutils.GetUserID() + "1"}, nil).Once()
				client.On("GetUser", testutils.GetUserID()+"2").Return(&clientmodels.User{ID: testutils.GetUserID() + "2"}, nil).Once()
			},
			expectedError: "dm/gm not found",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			testCase.setupStore(p.store.(*storemocks.Store))
			testCase.setupAPI(p.API.(*plugintest.API))
			testutils.MockLogs(p.API.(*plugintest.API))
			testCase.setupClient(p.msteamsAppClient.(*clientmocks.Client))

			ah := ActivityHandler{}
			ah.plugin = p

			resp, userIDs, err := ah.getChatChannelIDAndUsersID(testCase.chat)
			assert.Equal(t, resp, testCase.expectedResponse)
			assert.Equal(t, userIDs, testCase.expectedUserIDs)
			if testCase.expectedError != "" {
				assert.EqualError(t, err, testCase.expectedError)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestGetMessageFromChannel(t *testing.T) {
	for _, testCase := range []struct {
		description   string
		userID        string
		teamID        string
		channelID     string
		messageID     string
		expectedError string
		setupClient   func(client *clientmocks.Client)
	}{
		{
			description: "Successfully got message from channel",
			userID:      testutils.GetUserID(),
			teamID:      testutils.GetTeamsUserID(),
			channelID:   testutils.GetChannelID(),
			messageID:   testutils.GetMessageID(),
			setupClient: func(client *clientmocks.Client) {
				client.On("GetMessage", testutils.GetTeamsUserID(), testutils.GetChannelID(), testutils.GetMessageID()).Return(&clientmodels.Message{}, nil)
			},
		},
		{
			description: "Unable to get original post",
			userID:      testutils.GetUserID(),
			teamID:      testutils.GetTeamsUserID(),
			channelID:   testutils.GetChannelID(),
			messageID:   "mock-messageID",
			setupClient: func(client *clientmocks.Client) {
				client.On("GetMessage", testutils.GetTeamsUserID(), testutils.GetChannelID(), "mock-messageID").Return(nil, errors.New("Error while getting the original post"))
			},
			expectedError: "Error while getting the original post",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			testCase.setupClient(p.msteamsAppClient.(*clientmocks.Client))
			testutils.MockLogs(p.API.(*plugintest.API))
			ah := ActivityHandler{}
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
	for _, testCase := range []struct {
		description   string
		userID        string
		teamID        string
		channelID     string
		messageID     string
		replyID       string
		expectedError string
		setupClient   func(client *clientmocks.Client)
	}{
		{
			description: "Successfully got reply from channel",
			userID:      testutils.GetUserID(),
			teamID:      testutils.GetTeamsUserID(),
			channelID:   testutils.GetChannelID(),
			messageID:   testutils.GetMessageID(),
			replyID:     testutils.GetReplyID(),
			setupClient: func(client *clientmocks.Client) {
				client.On("GetReply", testutils.GetTeamsUserID(), testutils.GetChannelID(), testutils.GetMessageID(), testutils.GetReplyID()).Return(&clientmodels.Message{}, nil)
			},
		},
		{
			description: "Unable to get original post",
			userID:      testutils.GetUserID(),
			teamID:      testutils.GetTeamsUserID(),
			channelID:   testutils.GetChannelID(),
			messageID:   testutils.GetMessageID(),
			replyID:     "mock-replyID",
			setupClient: func(client *clientmocks.Client) {
				client.On("GetReply", testutils.GetTeamsUserID(), testutils.GetChannelID(), testutils.GetMessageID(), "mock-replyID").Return(nil, errors.New("Error while getting the original post"))
			},
			expectedError: "Error while getting the original post",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			testCase.setupClient(p.msteamsAppClient.(*clientmocks.Client))
			testutils.MockLogs(p.API.(*plugintest.API))
			ah := ActivityHandler{}
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
	for _, testCase := range []struct {
		description      string
		channelID        string
		teamID           string
		expectedResponse string
		setupStore       func(store *storemocks.Store)
	}{
		{
			description:      "Get bot user ID",
			teamID:           testutils.GetTeamsUserID(),
			expectedResponse: "bot-user-id",
			setupStore: func(store *storemocks.Store) {
				store.On("GetLinkByMSTeamsChannelID", testutils.GetTeamsUserID(), "").Return(nil, errors.New("Error while getting channel link"))
			},
		},
		{
			description:      "Get creator of channel link",
			teamID:           testutils.GetTeamsUserID(),
			channelID:        testutils.GetChannelID(),
			expectedResponse: "mock-creator",
			setupStore: func(store *storemocks.Store) {
				store.On("GetLinkByMSTeamsChannelID", testutils.GetTeamsUserID(), testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					Creator: "mock-creator",
				}, nil)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			testCase.setupStore(p.store.(*storemocks.Store))
			ah := ActivityHandler{}

			ah.plugin = p

			resp := ah.getUserIDForChannelLink(testCase.teamID, testCase.channelID)
			assert.Equal(t, resp, testCase.expectedResponse)
		})
	}
}

func TestGetMessageAndChatFromActivityIds(t *testing.T) {
	for _, testCase := range []struct {
		description   string
		activityIds   clientmodels.ActivityIds
		setupClient   func(*clientmocks.Client, *clientmocks.Client)
		setupStore    func(store *storemocks.Store)
		setupAPI      func(api *plugintest.API)
		setupMetrics  func(*metricsmocks.Metrics)
		expectedError string
	}{
		{
			description: "Successfully get message and chat from activity ID",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetUserID(),
						},
					},
					ID: testutils.GetChatID(),
				}, nil)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{}, nil)
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()).Return("mock-mmUserID", nil)
				store.On("GetTokenForMattermostUser", "mock-mmUserID").Return(&fakeToken, nil)
			},
			setupAPI: func(api *plugintest.API) {},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Unable to get original chat",
			activityIds: clientmodels.ActivityIds{
				ChatID: "mock-ChatID",
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", "mock-ChatID").Return(nil, errors.New("Error while getting original chat"))
			},
			setupStore: func(store *storemocks.Store) {},
			setupAPI: func(api *plugintest.API) {
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
			},
			expectedError: "Error while getting original chat",
		},
		{
			description: "Unable to get message from chat",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: "mock-MessageID",
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetUserID(),
						},
					},
					ID: testutils.GetChatID(),
				}, nil).Once()
				uclient.On("GetChatMessage", testutils.GetChatID(), "mock-MessageID").Return(nil, errors.New("Error while getting chat message")).Once()
			},
			setupAPI: func(api *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()).Return("mock-mmUserID", nil)
				store.On("GetTokenForMattermostUser", "mock-mmUserID").Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			expectedError: "Error while getting chat message",
		},
		{
			description: "Unable to get reply from channel",
			activityIds: clientmodels.ActivityIds{
				ReplyID:   testutils.GetReplyID(),
				MessageID: testutils.GetMessageID(),
				TeamID:    testutils.GetTeamsUserID(),
				ChannelID: testutils.GetChannelID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetReply", testutils.GetTeamsUserID(), testutils.GetChannelID(), testutils.GetMessageID(), testutils.GetReplyID()).Return(nil, errors.New("Error while getting reply from channel"))
			},
			setupAPI: func(api *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
			},
			expectedError: "Error while getting reply from channel",
		},
		{
			description: "Unable to get message from channel",
			activityIds: clientmodels.ActivityIds{
				MessageID: "mock-MessageID",
				TeamID:    testutils.GetTeamsUserID(),
				ChannelID: testutils.GetChannelID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetMessage", testutils.GetTeamsUserID(), testutils.GetChannelID(), "mock-MessageID").Return(nil, errors.New("Error while getting message from channel"))
			},
			setupAPI: func(api *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
			},
			expectedError: "Error while getting message from channel",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)

			testCase.setupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			testCase.setupStore(p.store.(*storemocks.Store))
			testCase.setupAPI(p.API.(*plugintest.API))
			testCase.setupMetrics(p.metricsService.(*metricsmocks.Metrics))
			testutils.MockLogs(p.API.(*plugintest.API))
			ah := ActivityHandler{}
			ah.plugin = p

			message, chat, err := ah.getMessageAndChatFromActivityIds(nil, testCase.activityIds)
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
