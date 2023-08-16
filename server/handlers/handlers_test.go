package handlers

import (
	"errors"
	"fmt"
	"testing"
	"time"

	mocksPlugin "github.com/mattermost/mattermost-plugin-msteams-sync/server/handlers/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	mocksClient "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	mocksStore "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/stretchr/testify/mock"
)

func TestHandleCreatedActivity(t *testing.T) {
	for _, testCase := range []struct {
		description string
		activityIds msteams.ActivityIds
		setupPlugin func(*mocksPlugin.PluginIface, *mocksClient.Client, *plugintest.API, *mocksStore.Store)
		setupClient func(*mocksClient.Client)
		setupAPI    func(*plugintest.API)
		setupStore  func(*mocksStore.Store)
	}{
		{
			description: "Unable to get original message",
			activityIds: msteams.ActivityIds{
				ChatID: "invalid-ChatID",
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(2)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", "invalid-ChatID").Return(nil, errors.New("Error while getting original chat")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogError", "Unable to get original chat", "chatID", "invalid-ChatID", "error", errors.New("Error while getting original chat")).Times(1)
				mockAPI.On("LogError", "Unable to get original message", "error", mock.Anything).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {},
		},
		{
			description: "Message is nil",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(2)
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Unable to get the message (probably because belongs to private chats of non-connected users)").Times(1)
				mockAPI.On("LogError", "Unable to get message from chat", "chatID", testutils.GetChatID(), "messageID", testutils.GetMessageID(), "error", nil).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {},
		},
		{
			description: "Skipping not user event",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Skipping not user event", "msg", &msteams.Message{}).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {},
		},
		{
			description: "Skipping messages from bot user",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(2)
				p.On("GetStore").Return(store).Times(1)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetTeamsUserID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(nil).Times(1)
				mockAPI.On("LogDebug", "Skipping messages from bot user").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
			},
		},
		{
			description: "Unable to get channel ID",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(3)
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(2)
				p.On("GetStore").Return(store).Times(1)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				p.On("GetSyncDirectMessages").Return(true).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&msteams.User{ID: testutils.GetSenderID()}, nil).Once()
				client.On("GetUser", testutils.GetTeamsUserID()).Return(nil, fmt.Errorf("unable to get user")).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogError", "Unable to get the MS Teams user", "error", "unable to get user").Times(1)
				mockAPI.On("LogError", "Unable to get original channel id", "error", "not enough users for creating a channel").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
			},
		},
		{
			description: "Channel ID is empty",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(4)
				p.On("GetClientForTeamsUser", "mockUserID-1").Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(3)
				p.On("GetStore").Return(store).Times(4)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				p.On("GetSyncDirectMessages").Return(true).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&msteams.User{ID: testutils.GetSenderID()}, nil).Once()
				client.On("GetUser", "mockUserID-1").Return(&msteams.User{ID: "mockUserID-1"}, nil).Once()
				client.On("GetUser", "mockUserID-2").Return(&msteams.User{ID: "mockUserID-2"}, nil).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetDirectChannel", "mockUserID-1", "mockUserID-2").Return(&model.Channel{}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("LogDebug", "Channel not set").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
			},
		},
		{
			description: "Duplicate post",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(4)
				p.On("GetClientForTeamsUser", "mockUserID-1").Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(5)
				p.On("GetStore").Return(store).Times(6)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(3)
				p.On("GetSyncDirectMessages").Return(true).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&msteams.User{ID: testutils.GetSenderID()}, nil).Once()
				client.On("GetUser", "mockUserID-1").Return(&msteams.User{ID: "mockUserID-1"}, nil).Once()
				client.On("GetUser", "mockUserID-2").Return(&msteams.User{ID: "mockUserID-2"}, nil).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetDirectChannel", "mockUserID-1", "mockUserID-2").Return(&model.Channel{Id: testutils.GetChannelID()}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("LogDebug", "Post generated", "post", mock.Anything).Times(1)
				mockAPI.On("LogDebug", "duplicate post").Times(1)
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(nil).Once()
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(2)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{}, nil).Times(1)
			},
		},
		{
			description: "Unable to create post",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(4)
				p.On("GetClientForTeamsUser", "mockUserID-1").Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(5)
				p.On("GetStore").Return(store).Times(6)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(3)
				p.On("GetSyncDirectMessages").Return(true).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&msteams.User{ID: testutils.GetSenderID()}, nil).Once()
				client.On("GetUser", "mockUserID-1").Return(&msteams.User{ID: "mockUserID-1"}, nil).Once()
				client.On("GetUser", "mockUserID-2").Return(&msteams.User{ID: "mockUserID-2"}, nil).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetDirectChannel", "mockUserID-1", "mockUserID-2").Return(&model.Channel{Id: testutils.GetChannelID()}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("LogDebug", "Post generated", "post", mock.Anything).Times(1)
				mockAPI.On("CreatePost", testutils.GetPostFromTeamsMessage()).Return(nil, testutils.GetInternalServerAppError("unable to create the post")).Times(1)
				mockAPI.On("LogError", "Unable to create post", "post", mock.Anything, "error", mock.Anything).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(2)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
			},
		},
		{
			description: "Error updating the post",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(4)
				p.On("GetClientForTeamsUser", "mockUserID-1").Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(7)
				p.On("GetStore").Return(store).Times(7)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(3)
				p.On("GetSyncDirectMessages").Return(true).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&msteams.User{ID: testutils.GetSenderID()}, nil).Once()
				client.On("GetUser", "mockUserID-1").Return(&msteams.User{ID: "mockUserID-1"}, nil).Once()
				client.On("GetUser", "mockUserID-2").Return(&msteams.User{ID: "mockUserID-2"}, nil).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetDirectChannel", "mockUserID-1", "mockUserID-2").Return(&model.Channel{Id: testutils.GetChannelID()}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("LogDebug", "Post generated", "post", mock.Anything).Times(1)
				mockAPI.On("CreatePost", testutils.GetPostFromTeamsMessage()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				mockAPI.On("LogDebug", "Post created", "post", mock.Anything).Times(1)
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(nil).Times(1)
				mockAPI.On("LogWarn", "Error updating the MSTeams/Mattermost post link metadata", "error", mock.Anything).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(2)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      testutils.GetMessageID(),
					MSTeamsChannel: testutils.GetMSTeamsChannelID(),
				}).Return(errors.New("unable to update the post")).Times(1)
			},
		},
		{
			description: "Valid: chat message",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(4)
				p.On("GetClientForTeamsUser", "mockUserID-1").Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(6)
				p.On("GetStore").Return(store).Times(7)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(3)
				p.On("GetSyncDirectMessages").Return(true).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&msteams.User{ID: testutils.GetSenderID()}, nil).Once()
				client.On("GetUser", "mockUserID-1").Return(&msteams.User{ID: "mockUserID-1"}, nil).Once()
				client.On("GetUser", "mockUserID-2").Return(&msteams.User{ID: "mockUserID-2"}, nil).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetDirectChannel", "mockUserID-1", "mockUserID-2").Return(&model.Channel{Id: testutils.GetChannelID()}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("LogDebug", "Post generated", "post", mock.Anything).Times(1)
				mockAPI.On("CreatePost", testutils.GetPostFromTeamsMessage()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				mockAPI.On("LogDebug", "Post created", "post", mock.Anything).Times(1)
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(nil).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(2)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      testutils.GetMessageID(),
					MSTeamsChannel: testutils.GetMSTeamsChannelID(),
				}).Return(nil).Times(1)
			},
		},
		{
			description: "Valid: channel message",
			activityIds: msteams.ActivityIds{
				TeamID:    "mockTeamID",
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForUser", "mockCreator").Return(client, nil).Times(1)
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(5)
				p.On("GetStore").Return(store).Times(7)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(3)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetMessage", "mockTeamID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					TeamID:          "mockTeamID",
					ChannelID:       testutils.GetChannelID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&msteams.User{ID: testutils.GetSenderID()}, nil).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("LogDebug", "Post generated", "post", mock.Anything).Times(1)
				mockAPI.On("CreatePost", testutils.GetPostFromTeamsMessage()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				mockAPI.On("LogDebug", "Post created", "post", mock.Anything).Times(1)
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(nil).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("GetLinkByMSTeamsChannelID", "mockTeamID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					Creator:             "mockCreator",
					MattermostChannelID: testutils.GetChannelID(),
				}, nil).Times(3)
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      testutils.GetMessageID(),
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			ah := ActivityHandler{}
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			testCase.setupPlugin(p, client, mockAPI, store)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(store)

			ah.plugin = p

			ah.handleCreatedActivity(testCase.activityIds)
		})
	}
}

func TestHandleUpdatedActivity(t *testing.T) {
	for _, testCase := range []struct {
		description string
		activityIds msteams.ActivityIds
		setupPlugin func(*mocksPlugin.PluginIface, *mocksClient.Client, *plugintest.API, *mocksStore.Store)
		setupClient func(*mocksClient.Client)
		setupAPI    func(*plugintest.API)
		setupStore  func(*mocksStore.Store)
	}{
		{
			description: "Unable to get original message",
			activityIds: msteams.ActivityIds{
				ChatID: "invalid-ChatID",
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(2)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", "invalid-ChatID").Return(nil, errors.New("error while getting original chat")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogError", "Unable to get original chat", "chatID", "invalid-ChatID", "error", errors.New("error while getting original chat")).Times(1)
				mockAPI.On("LogError", "Unable to get original message", "error", mock.Anything).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {},
		},
		{
			description: "Message is nil",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(2)
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Unable to get the message (probably because belongs to private chats of non-connected users)").Times(1)
				mockAPI.On("LogError", "Unable to get message from chat", "chatID", testutils.GetChatID(), "messageID", testutils.GetMessageID(), "error", nil).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {},
		},
		{
			description: "Skipping not user event",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{}, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Skipping not user event", "msg", &msteams.Message{}).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {},
		},
		{
			description: "Skipping messages from bot user",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(2)
				p.On("GetStore").Return(store).Times(1)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(nil).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetTeamsUserID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Skipping messages from bot user").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
			},
		},
		{
			description: "Unable to get the post info",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(1)
				p.On("GetStore").Return(store).Times(2)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(nil).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
			},
		},
		{
			description: "Unable to get the post",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(2)
				p.On("GetStore").Return(store).Times(2)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(nil).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetPost", "mockMattermostID").Return(nil, testutils.GetInternalServerAppError("unable to get the post")).Times(1)
				mockAPI.On("LogError", "Unable to find the original post", "error", mock.Anything).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID:        "mockMattermostID",
					MSTeamsID:           "mockMSTeamsID",
					MSTeamsChannel:      "mockMSTeamsChannel",
					MSTeamsLastUpdateAt: time.Now(),
				}, nil).Times(1)
			},
		},
		{
			description: "Unable to get and recover the post",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(2)
				p.On("GetStore").Return(store).Times(3)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(nil).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				getPostError := testutils.GetInternalServerAppError("Unable to get the post.")
				getPostError.Id = "app.post.get.app_error"
				mockAPI.On("GetPost", "mockMattermostID").Return(nil, getPostError).Times(1)
				mockAPI.On("LogError", "Unable to recover the post", "postID", "mockMattermostID", "error", errors.New("unable to recover")).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID:        "mockMattermostID",
					MSTeamsID:           "mockMSTeamsID",
					MSTeamsChannel:      "mockMSTeamsChannel",
					MSTeamsLastUpdateAt: time.Now(),
				}, nil).Times(1)
				store.On("RecoverPost", "mockMattermostID").Return(errors.New("unable to recover"))
			},
		},
		{
			description: "Valid: chat message",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client).Times(1)
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(6)
				p.On("GetStore").Return(store).Times(4)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(nil).Times(1)
				p.On("GetBotUserID").Return(testutils.GetSenderID()).Times(2)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&msteams.Chat{
					ID: testutils.GetChatID(),
					Members: []msteams.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetPost", "mockMattermostID").Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetSenderID()), nil).Times(1)
				mockAPI.On("LogDebug", "Handling reactions", "reactions", mock.Anything).Times(1)
				mockAPI.On("UpdatePost", mock.Anything).Return(nil, nil).Times(1)
				mockAPI.On("GetReactions", "mockMattermostID").Return([]*model.Reaction{}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetTeamsUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID:        "mockMattermostID",
					MSTeamsID:           "mockMSTeamsID",
					MSTeamsChannel:      "mockMSTeamsChannel",
					MSTeamsLastUpdateAt: time.Now(),
				}, nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetTeamsUserID(), nil).Times(2)
			},
		},
		{
			description: "Valid: channel message",
			activityIds: msteams.ActivityIds{
				TeamID:    "mockTeamID",
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetClientForUser", "mockCreator").Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(5)
				p.On("GetStore").Return(store).Times(6)
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(nil).Times(1)
				p.On("GetBotUserID").Return(testutils.GetSenderID()).Times(2)
				p.On("GetSyncDirectMessages").Return(true).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetMessage", "mockTeamID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&msteams.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					TeamID:          "mockTeamID",
					ChannelID:       testutils.GetChannelID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetPost", "mockMattermostID").Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetSenderID()), nil).Times(1)
				mockAPI.On("LogDebug", "Handling reactions", "reactions", mock.Anything).Times(1)
				mockAPI.On("LogError", "Message reactions", "reactions", mock.Anything, "error", mock.Anything).Times(1)
				mockAPI.On("UpdatePost", mock.Anything).Return(nil, nil).Times(1)
				mockAPI.On("GetReactions", "mockMattermostID").Return([]*model.Reaction{}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MSTeamsLastUpdateAt: time.Now(),
					MattermostID:        "mockMattermostID",
				}, nil).Times(1)
				store.On("GetLinkByMSTeamsChannelID", "mockTeamID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					Creator: "mockCreator",
				}, nil).Times(1)
				store.On("GetLinkByMSTeamsChannelID", "mockTeamID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					Creator:             "mockCreator",
					MattermostChannelID: testutils.GetChannelID(),
				}, nil).Times(2)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			ah := ActivityHandler{}
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			testCase.setupPlugin(p, client, mockAPI, store)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(store)

			ah.plugin = p
			ah.handleUpdatedActivity(testCase.activityIds)
		})
	}
}

func TestHandleDeletedActivity(t *testing.T) {
	for _, testCase := range []struct {
		description string
		activityIds msteams.ActivityIds
		setupPlugin func(*mocksPlugin.PluginIface, *plugintest.API, *mocksStore.Store)
		setupAPI    func(*plugintest.API)
		setupStore  func(*mocksStore.Store)
	}{
		{
			description: "Successfully deleted post",
			activityIds: msteams.ActivityIds{
				ChatID:    testutils.GetChatID(),
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetStore").Return(store)
				p.On("GetAPI").Return(mockAPI)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("DeletePost", testutils.GetMattermostID()).Return(nil)
				mockAPI.On("LogError", "Unable to to delete post", "msgID", "", "error", &model.AppError{
					Message: "Error while deleting a post",
				})
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("GetPostInfoByMSTeamsID", fmt.Sprintf("%s%s", testutils.GetChatID(), testutils.GetChannelID()), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetMattermostID(),
				}, nil)
			},
		},
		{
			description: "Unable to get post info by MS teams ID",
			activityIds: msteams.ActivityIds{
				ChannelID: testutils.GetChannelID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetStore").Return(store)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), "").Return(nil, errors.New("Error while getting post info by MS teams ID"))
			},
		},
		{
			description: "Unable to to delete post",
			activityIds: msteams.ActivityIds{
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetStore").Return(store)
				p.On("GetAPI").Return(mockAPI)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("DeletePost", "").Return(&model.AppError{
					Message: "Error while deleting a post",
				})
				mockAPI.On("LogError", "Unable to to delete post", "msgID", "", "error", &model.AppError{
					Message: "Error while deleting a post",
				})
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{}, nil)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			ah := ActivityHandler{}
			mockAPI := &plugintest.API{}
			store := mocksStore.NewStore(t)
			ah.plugin = p

			testCase.setupPlugin(p, mockAPI, store)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(store)

			ah.handleDeletedActivity(testCase.activityIds)
		})
	}
}

func TestHandleReactions(t *testing.T) {
	for _, testCase := range []struct {
		description string
		reactions   []msteams.Reaction
		setupPlugin func(*mocksPlugin.PluginIface, *plugintest.API, *mocksStore.Store)
		setupAPI    func(*plugintest.API)
		setupStore  func(*mocksStore.Store)
	}{
		{
			description: "Reactions list is empty",
			reactions:   []msteams.Reaction{},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetAPI").Return(mockAPI).Times(2)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetReactions", testutils.GetPostID()).Return([]*model.Reaction{}, nil).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {},
		},
		{
			description: "Unable to get the reactions",
			reactions: []msteams.Reaction{
				{
					UserID:   testutils.GetTeamsUserID(),
					Reaction: "+1",
				},
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetAPI").Return(mockAPI).Times(2)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetReactions", testutils.GetPostID()).Return(nil, testutils.GetInternalServerAppError("unable to get the reaction")).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {},
		},
		{
			description: "Unable to find the user for the reaction",
			reactions: []msteams.Reaction{
				{
					UserID:   testutils.GetTeamsUserID(),
					Reaction: "+1",
				},
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetStore").Return(store).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(5)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetReactions", testutils.GetPostID()).Return([]*model.Reaction{
					{
						UserId:    testutils.GetTeamsUserID(),
						EmojiName: "+1",
						PostId:    testutils.GetPostID(),
					},
				}, nil).Times(1)

				mockAPI.On("RemoveReaction", &model.Reaction{
					UserId:    testutils.GetTeamsUserID(),
					EmojiName: "+1",
					PostId:    testutils.GetPostID(),
					ChannelId: "removedfromplugin",
				}).Return(nil).Times(1)

				mockAPI.On("LogError", "No code reaction found for reaction", "reaction", mock.Anything).Times(1)
				mockAPI.On("LogError", "unable to find the user for the reaction", "reaction", mock.Anything).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return("", errors.New("unable to find the user for the reaction")).Times(1)
			},
		},
		{
			description: "Unable to remove the reaction",
			reactions: []msteams.Reaction{
				{
					UserID:   testutils.GetTeamsUserID(),
					Reaction: "+1",
				},
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetStore").Return(store).Times(1)
				p.On("GetAPI").Return(mockAPI).Times(6)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetReactions", testutils.GetPostID()).Return([]*model.Reaction{
					{
						UserId:    testutils.GetTeamsUserID(),
						EmojiName: "+1",
						PostId:    testutils.GetPostID(),
					},
				}, nil).Times(1)

				mockAPI.On("RemoveReaction", &model.Reaction{
					UserId:    testutils.GetTeamsUserID(),
					EmojiName: "+1",
					PostId:    testutils.GetPostID(),
					ChannelId: "removedfromplugin",
				}).Return(testutils.GetInternalServerAppError("unable to remove reaction")).Times(1)

				mockAPI.On("LogError", "No code reaction found for reaction", "reaction", mock.Anything).Times(2)
				mockAPI.On("LogError", "Unable to remove reaction", "error", mock.Anything).Times(1)
				mockAPI.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetID(), nil).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			ah := ActivityHandler{}
			mockAPI := &plugintest.API{}
			store := mocksStore.NewStore(t)
			ah.plugin = p

			testCase.setupPlugin(p, mockAPI, store)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(store)

			mockAPI.On("LogDebug", "Handling reactions", "reactions", mock.Anything).Times(1)

			ah.handleReactions(testutils.GetPostID(), testutils.GetChannelID(), testCase.reactions)
		})
	}
}

func TestUpdateLastReceivedChangeDate(t *testing.T) {
	for _, testCase := range []struct {
		description string
		setupPlugin func(*mocksPlugin.PluginIface, *plugintest.API)
		setupAPI    func(*plugintest.API)
	}{
		{
			description: "Unable to set the value in kv store",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API) {
				p.On("GetAPI").Return(mockAPI).Times(2)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(testutils.GetInternalServerAppError("unable to set the value in kv store")).Times(1)
				mockAPI.On("LogError", "Unable to store properly the last received change").Times(1)
			},
		},
		{
			description: "Valid",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API) {
				p.On("GetAPI").Return(mockAPI).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("KVSet", lastReceivedChangeKey, mock.Anything).Return(nil).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			ah := ActivityHandler{}
			mockAPI := &plugintest.API{}
			ah.plugin = p

			testCase.setupPlugin(p, mockAPI)
			testCase.setupAPI(mockAPI)

			ah.updateLastReceivedChangeDate(time.Time{})
		})
	}
}
