package handlers

import (
	"errors"
	"testing"

	mocksPlugin "github.com/mattermost/mattermost-plugin-msteams-sync/server/handlers/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	mocksStore "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/stretchr/testify/assert"

	mocksClient "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest/mock"
)

func TestGetChatChannelID(t *testing.T) {
	ah := ActivityHandler{}
	mockAPI := &plugintest.API{}
	store := mocksStore.NewStore(t)

	for _, testCase := range []struct {
		description      string
		chat             *msteams.Chat
		messageID        string
		expectedResponse string
		expectedError    string
		setupPlugin      func(plugin *mocksPlugin.PluginIface)
		setupAPI         func()
		setupStore       func()
	}{
		{
			description: "Successfully got the ID of direct channel",
			chat: &msteams.Chat{
				Members: []msteams.ChatMember{
					{
						UserID: testutils.GetUserID() + "1",
					},
					{
						UserID: testutils.GetUserID() + "2",
					},
				},
				Type: "D",
			},
			messageID: testutils.GetMessageID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
				p.On("GetAPI").Return(mockAPI)
			},
			setupAPI: func() {
				mockAPI.On("GetDirectChannel", "mock-mmUserID1", "mock-mmUserID2").Return(&model.Channel{
					Id: testutils.GetChannelID(),
				}, nil)
			},
			setupStore: func() {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"1").Return("mock-mmUserID1", nil)
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"2").Return("mock-mmUserID2", nil)
			},
			expectedResponse: testutils.GetChannelID(),
		},
		{
			description: "Successfully got the ID of group channel",
			chat: &msteams.Chat{
				Members: []msteams.ChatMember{
					{
						UserID: testutils.GetUserID() + "1",
					},
					{
						UserID: testutils.GetUserID() + "2",
					},
				},
				Type: "G",
			},
			messageID: testutils.GetMessageID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
				p.On("GetAPI").Return(mockAPI)
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
			expectedResponse: testutils.GetChannelID(),
		},
		{
			description: "Unable to get or create synthetic user",
			chat: &msteams.Chat{
				Members: []msteams.ChatMember{
					{
						UserID: "mock-userID",
					},
				},
			},
			messageID: testutils.GetMessageID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
				p.On("GetAPI").Return(mockAPI)
			},
			setupAPI: func() {
				mockAPI.On("GetUserByEmail", "mock-userID@msteamssync").Return(&model.User{
					Id: "mock-Id",
				}, nil)
			},
			setupStore: func() {
				var token *msteams.Token
				store.On("TeamsToMattermostUserID", "mock-userID").Return("", errors.New("Error while getting mattermost userID"))
				store.On("SetUserInfo", "mock-Id", "mock-userID", token).Return(errors.New("Error while setting user info"))
			},
			expectedError:    "Error while setting user info",
			expectedResponse: "",
		},
		{
			description: "Not enough user for creating a channel",
			chat: &msteams.Chat{
				Members: []msteams.ChatMember{
					{
						UserID: testutils.GetUserID(),
					},
				},
			},
			messageID: testutils.GetMessageID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
			},
			setupAPI: func() {},
			setupStore: func() {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()).Return("mock-mmUserID", nil)
			},
			expectedError:    "not enough user for creating a channel",
			expectedResponse: "",
		},
		{
			description: "Direct or group channel not found",
			chat: &msteams.Chat{
				Members: []msteams.ChatMember{
					{
						UserID: testutils.GetUserID() + "1",
					},
					{
						UserID: testutils.GetUserID() + "2",
					},
				},
			},
			messageID: testutils.GetMessageID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
			},
			setupAPI: func() {},
			setupStore: func() {
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"1").Return("mock-mmUserID1", nil)
				store.On("TeamsToMattermostUserID", testutils.GetUserID()+"2").Return("mock-mmUserID2", nil)
			},
			expectedError: "dm/gm not found",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			mockAPI.On("LogError", mock.Anything, mock.Anything, mock.Anything)
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			testCase.setupStore()
			testCase.setupAPI()
			ah.plugin = p

			resp, err := ah.getChatChannelID(testCase.chat, testCase.messageID)
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
			description: "Successfully got reply from channel",
			userID:      testutils.GetUserID(),
			teamID:      testutils.GetTeamUserID(),
			channelID:   testutils.GetChannelID(),
			messageID:   testutils.GetMessageID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
			},
			setupClient: func() {
				client.On("GetMessage", testutils.GetTeamUserID(), testutils.GetChannelID(), testutils.GetMessageID()).Return(&msteams.Message{}, nil)
			},
		},
		{
			description: "Unable to get bot client",
			userID:      "mock-userID",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForUser", "mock-userID").Return(nil, errors.New("Error while getting bot client"))
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient:   func() {},
			expectedError: "Error while getting bot client",
		},
		{
			description: "Unable to get original post",
			userID:      testutils.GetUserID(),
			teamID:      testutils.GetTeamUserID(),
			channelID:   testutils.GetChannelID(),
			messageID:   "mock-messageID",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func() {
				client.On("GetMessage", testutils.GetTeamUserID(), testutils.GetChannelID(), "mock-messageID").Return(nil, errors.New("Error while getting the original post"))
			},
			expectedError: "Error while getting the original post",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			mockAPI.On("LogError", mock.Anything, mock.Anything, mock.Anything)
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			testCase.setupClient()
			ah.plugin = p

			resp, err := ah.getMessageFromChannel(testCase.userID, testCase.teamID, testCase.channelID, testCase.messageID)
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
			teamID:      testutils.GetTeamUserID(),
			channelID:   testutils.GetChannelID(),
			messageID:   testutils.GetMessageID(),
			replyID:     testutils.GetReplyID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
			},
			setupClient: func() {
				client.On("GetReply", testutils.GetTeamUserID(), testutils.GetChannelID(), testutils.GetMessageID(), testutils.GetReplyID()).Return(&msteams.Message{}, nil)
			},
		},
		{
			description: "Unable to get bot client",
			userID:      "mock-userID",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForUser", "mock-userID").Return(nil, errors.New("Error while getting bot client"))
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient:   func() {},
			expectedError: "Error while getting bot client",
		},
		{
			description: "Unable to get original post",
			userID:      testutils.GetUserID(),
			teamID:      testutils.GetTeamUserID(),
			channelID:   testutils.GetChannelID(),
			messageID:   testutils.GetMessageID(),
			replyID:     "mock-replyID",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func() {
				client.On("GetReply", testutils.GetTeamUserID(), testutils.GetChannelID(), testutils.GetMessageID(), "mock-replyID").Return(nil, errors.New("Error while getting the original post"))
			},
			expectedError: "Error while getting the original post",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			mockAPI.On("LogError", mock.Anything, mock.Anything, mock.Anything)
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			testCase.setupClient()
			ah.plugin = p

			resp, err := ah.getReplyFromChannel(testCase.userID, testCase.teamID, testCase.channelID, testCase.messageID, testCase.replyID)
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
	store := mocksStore.NewStore(t)

	for _, testCase := range []struct {
		description      string
		channelID        string
		teamID           string
		expectedResponse string
		setupPlugin      func(plugin *mocksPlugin.PluginIface)
	}{
		{
			description:      "Get bot user ID",
			teamID:           testutils.GetTeamUserID(),
			expectedResponse: testutils.GetUserID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
				p.On("GetBotUserID").Return(testutils.GetUserID())
				store.On("GetLinkByMSTeamsChannelID", testutils.GetTeamUserID(), "").Return(nil, errors.New("Error while getting channel link"))
			},
		},
		{
			description:      "Get creator of channel link",
			teamID:           testutils.GetTeamUserID(),
			channelID:        testutils.GetChannelID(),
			expectedResponse: "mock-creator",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
				store.On("GetLinkByMSTeamsChannelID", testutils.GetTeamUserID(), testutils.GetChannelID()).Return(&storemodels.ChannelLink{
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
	client := mocksClient.NewClient(t)
	store := mocksStore.NewStore(t)
	mockAPI := &plugintest.API{}

	for _, testCase := range []struct {
		description   string
		activityIds   msteams.ActivityIds
		setupPlugin   func(plugin *mocksPlugin.PluginIface)
		setupClient   func()
		setupStore    func()
		expectedError string
	}{
		{
			description: "Successfully get message and chat from activity ID",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForApp").Return(client)
				p.On("GetClientForTeamsUser", testutils.GetUserID()).Return(client, nil)
			},
			setupClient: func() {
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
			setupStore: func() {},
		},
		{
			description: "Unable to get original chat",
			activityIds: msteams.ActivityIds{
				ChatID: "mock-ChatID",
			},
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForApp").Return(client)
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func() {
				client.On("GetChat", "mock-ChatID").Return(nil, errors.New("Error while getting original chat"))
			},
			setupStore:    func() {},
			expectedError: "Error while getting original chat",
		},
		{
			description: "Unable to get message from chat",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: "mock-MessageID",
			},
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForApp").Return(client)
				p.On("GetClientForTeamsUser", testutils.GetUserID()).Return(client, nil)
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func() {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetUserID(),
						},
					},
					ID: testutils.GetChatID(),
				}, nil)
				client.On("GetChatMessage", testutils.GetChatID(), "mock-MessageID").Return(nil, errors.New("Error while getting chat message"))
			},
			setupStore:    func() {},
			expectedError: "Error while getting chat message",
		},
		{
			description: "Unable to get reply from channel",
			activityIds: msteams.ActivityIds{
				ReplyID:   testutils.GetReplyID(),
				MessageID: testutils.GetMessageID(),
				TeamID:    testutils.GetTeamUserID(),
				ChannelID: testutils.GetChannelID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func() {
				client.On("GetReply", testutils.GetTeamUserID(), testutils.GetChannelID(), testutils.GetMessageID(), testutils.GetReplyID()).Return(nil, errors.New("Error while getting reply from channel"))
			},
			setupStore: func() {
				store.On("GetLinkByMSTeamsChannelID", testutils.GetTeamUserID(), testutils.GetChannelID()).Return(&storemodels.ChannelLink{Creator: testutils.GetUserID()}, nil)
			},
			expectedError: "Error while getting reply from channel",
		},
		{
			description: "Unable to get message from channel",
			activityIds: msteams.ActivityIds{
				MessageID: "mock-MessageID",
				TeamID:    testutils.GetTeamUserID(),
				ChannelID: testutils.GetChannelID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func() {
				client.On("GetMessage", testutils.GetTeamUserID(), testutils.GetChannelID(), "mock-MessageID").Return(nil, errors.New("Error while getting message from channel"))
			},
			setupStore: func() {
				store.On("GetLinkByMSTeamsChannelID", testutils.GetTeamUserID(), testutils.GetChannelID()).Return(&storemodels.ChannelLink{Creator: testutils.GetUserID()}, nil)
			},
			expectedError: "Error while getting message from channel",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			mockAPI.On("LogError", mock.Anything, mock.Anything, mock.Anything)
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			testCase.setupClient()
			testCase.setupStore()
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
