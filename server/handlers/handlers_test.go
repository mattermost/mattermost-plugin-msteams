package handlers

import (
	"errors"
	"fmt"
	"testing"
	"time"

	mocksPlugin "github.com/mattermost/mattermost-plugin-msteams/server/handlers/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	mocksMetrics "github.com/mattermost/mattermost-plugin-msteams/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	mocksClient "github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	mocksStore "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleCreatedActivity(t *testing.T) {
	msteamsCreateAtTime := time.Now()
	mmCreateAtTime := msteamsCreateAtTime.UnixNano() / int64(time.Millisecond)
	for _, testCase := range []struct {
		description  string
		activityIds  clientmodels.ActivityIds
		setupPlugin  func(*mocksPlugin.PluginIface, *mocksClient.Client, *plugintest.API, *mocksStore.Store, *mocksMetrics.Metrics)
		setupClient  func(*mocksClient.Client)
		setupAPI     func(*plugintest.API)
		setupStore   func(*mocksStore.Store)
		setupMetrics func(*mocksMetrics.Metrics)
	}{
		{
			description: "Unable to get original message",
			activityIds: clientmodels.ActivityIds{
				ChatID: "invalid-ChatID",
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", "invalid-ChatID").Return(nil, errors.New("Error while getting original chat")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Message is nil",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Skipping not user event",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Duplicate post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					CreateAt:        msteamsCreateAtTime,
					LastUpdateAt:    msteamsCreateAtTime,
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveConfirmedMessage", metrics.ActionSourceMattermost, true).Times(1)
			},
		},
		{
			description: "Skipping messages from bot user",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetTeamsUserID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					CreateAt:        msteamsCreateAtTime,
					LastUpdateAt:    msteamsCreateAtTime,
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Unable to get channel ID",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				p.On("GetSyncDirectMessages").Return(true).Times(1)
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					CreateAt:        msteamsCreateAtTime,
					LastUpdateAt:    msteamsCreateAtTime,
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&clientmodels.User{ID: testutils.GetSenderID()}, nil).Once()
				client.On("GetUser", testutils.GetTeamsUserID()).Return(nil, fmt.Errorf("unable to get user")).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Channel ID is empty",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", "mockUserID-1").Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				p.On("GetSyncDirectMessages").Return(true).Times(1)
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					CreateAt:        msteamsCreateAtTime,
					LastUpdateAt:    msteamsCreateAtTime,
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&clientmodels.User{ID: testutils.GetSenderID()}, nil).Once()
				client.On("GetUser", "mockUserID-1").Return(&clientmodels.User{ID: "mockUserID-1"}, nil).Once()
				client.On("GetUser", "mockUserID-2").Return(&clientmodels.User{ID: "mockUserID-2"}, nil).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetDirectChannel", "mockUserID-1", "mockUserID-2").Return(&model.Channel{}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Unable to create post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", "mockUserID-1").Return(client, nil).Times(2)
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(3)
				p.On("GetSyncDirectMessages").Return(true).Times(1)
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					CreateAt:        msteamsCreateAtTime,
					LastUpdateAt:    msteamsCreateAtTime,
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&clientmodels.User{ID: testutils.GetSenderID()}, nil).Once()
				client.On("GetUser", "mockUserID-1").Return(&clientmodels.User{ID: "mockUserID-1"}, nil).Once()
				client.On("GetUser", "mockUserID-2").Return(&clientmodels.User{ID: "mockUserID-2"}, nil).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetDirectChannel", "mockUserID-1", "mockUserID-2").Return(&model.Channel{Id: testutils.GetChannelID()}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("CreatePost", testutils.GetPostFromTeamsMessage(mmCreateAtTime)).Return(nil, testutils.GetInternalServerAppError("unable to create the post")).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Error updating the post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", "mockUserID-1").Return(client, nil).Times(2)
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(3)
				p.On("GetSyncDirectMessages").Return(true).Times(1)
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					CreateAt:        msteamsCreateAtTime,
					LastUpdateAt:    msteamsCreateAtTime,
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&clientmodels.User{ID: testutils.GetSenderID()}, nil).Once()
				client.On("GetUser", "mockUserID-1").Return(&clientmodels.User{ID: "mockUserID-1"}, nil).Once()
				client.On("GetUser", "mockUserID-2").Return(&clientmodels.User{ID: "mockUserID-2"}, nil).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetDirectChannel", "mockUserID-1", "mockUserID-2").Return(&model.Channel{Id: testutils.GetChannelID()}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("CreatePost", testutils.GetPostFromTeamsMessage(mmCreateAtTime)).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), mmCreateAtTime), nil).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:        testutils.GetID(),
					MSTeamsID:           testutils.GetMessageID(),
					MSTeamsChannel:      testutils.GetMSTeamsChannelID(),
					MSTeamsLastUpdateAt: msteamsCreateAtTime,
				}).Return(errors.New("unable to update the post")).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMSTeams, true).Times(1)
			},
		},
		{
			description: "Valid: chat message",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", "mockUserID-1").Return(client, nil).Times(2)
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(3)
				p.On("GetSyncDirectMessages").Return(true).Times(1)
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					CreateAt:        msteamsCreateAtTime,
					LastUpdateAt:    msteamsCreateAtTime,
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&clientmodels.User{ID: testutils.GetSenderID()}, nil).Once()
				client.On("GetUser", "mockUserID-1").Return(&clientmodels.User{ID: "mockUserID-1"}, nil).Once()
				client.On("GetUser", "mockUserID-2").Return(&clientmodels.User{ID: "mockUserID-2"}, nil).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetDirectChannel", "mockUserID-1", "mockUserID-2").Return(&model.Channel{Id: testutils.GetChannelID()}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("CreatePost", testutils.GetPostFromTeamsMessage(mmCreateAtTime)).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), mmCreateAtTime), nil).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:        testutils.GetID(),
					MSTeamsID:           testutils.GetMessageID(),
					MSTeamsChannel:      testutils.GetMSTeamsChannelID(),
					MSTeamsLastUpdateAt: msteamsCreateAtTime,
				}).Return(nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMSTeams, true).Times(1)
			},
		},
		{
			description: "Valid: sync linked channels disabled",
			activityIds: clientmodels.ActivityIds{
				TeamID:    "mockTeamID",
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncLinkedChannels").Return(false).Times(1)
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetMessage", "mockTeamID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					TeamID:          "mockTeamID",
					ChannelID:       testutils.GetChannelID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					CreateAt:        msteamsCreateAtTime,
					LastUpdateAt:    msteamsCreateAtTime,
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&clientmodels.User{ID: testutils.GetSenderID()}, nil).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("CreatePost", testutils.GetPostFromTeamsMessage(mmCreateAtTime)).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), mmCreateAtTime), nil).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
			},
		},
		{
			description: "Valid: channel message",
			activityIds: clientmodels.ActivityIds{
				TeamID:    "mockTeamID",
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncLinkedChannels").Return(true).Times(1)
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(3)
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetMessage", "mockTeamID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					TeamID:          "mockTeamID",
					ChannelID:       testutils.GetChannelID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					CreateAt:        msteamsCreateAtTime,
					LastUpdateAt:    msteamsCreateAtTime,
				}, nil).Times(1)
				client.On("GetUser", testutils.GetSenderID()).Return(&clientmodels.User{ID: testutils.GetSenderID()}, nil).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("CreatePost", testutils.GetPostFromTeamsMessage(mmCreateAtTime)).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), mmCreateAtTime), nil).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetLinkByMSTeamsChannelID", "mockTeamID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					Creator:             "mockCreator",
					MattermostChannelID: testutils.GetChannelID(),
				}, nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:        testutils.GetID(),
					MSTeamsID:           testutils.GetMessageID(),
					MSTeamsChannel:      testutils.GetChannelID(),
					MSTeamsLastUpdateAt: msteamsCreateAtTime,
				}).Return(nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMSTeams, false).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			ah := ActivityHandler{}
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			mockmetrics := mocksMetrics.NewMetrics(t)
			testCase.setupPlugin(p, client, mockAPI, store, mockmetrics)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testutils.MockLogs(mockAPI)
			testCase.setupStore(store)
			testCase.setupMetrics(mockmetrics)
			subscriptionID := "test"

			ah.plugin = p

			discardedReason := ah.handleCreatedActivity(nil, subscriptionID, testCase.activityIds)

			lastSubscriptionActivity, ok := ah.lastUpdateAtMap.Load(subscriptionID)
			if discardedReason == "" {
				assert.True(t, ok)
				assert.Equal(t, lastSubscriptionActivity.(time.Time), msteamsCreateAtTime)
			} else {
				assert.False(t, ok)
			}
		})
	}
}

func TestHandleUpdatedActivity(t *testing.T) {
	msTeamsLastUpdateAtTime := time.Now()
	for _, testCase := range []struct {
		description  string
		activityIds  clientmodels.ActivityIds
		setupPlugin  func(*mocksPlugin.PluginIface, *mocksClient.Client, *plugintest.API, *mocksStore.Store, *mocksMetrics.Metrics)
		setupClient  func(*mocksClient.Client)
		setupAPI     func(*plugintest.API)
		setupStore   func(*mocksStore.Store)
		setupMetrics func(*mocksMetrics.Metrics)
	}{
		{
			description: "Unable to get original message",
			activityIds: clientmodels.ActivityIds{
				ChatID: "invalid-ChatID",
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(true).Maybe()
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", "invalid-ChatID").Return(nil, errors.New("error while getting original chat")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Message is nil",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(true).Maybe()
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Skipping not user event",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(true).Maybe()
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{}, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Skipping messages from bot user",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(true).Maybe()
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetTeamsUserID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Unable to get the post info",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(true).Maybe()
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Unable to get the post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(true).Maybe()
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				p.On("GetSyncDirectMessages").Return(true).Once()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetPost", "mockMattermostID").Return(nil, testutils.GetInternalServerAppError("unable to get the post")).Times(1)
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
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Unable to get and recover the post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(true).Maybe()
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(1)
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				p.On("GetSyncDirectMessages").Return(true).Once()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
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
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Valid: chat message",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(true).Maybe()
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetClientForTeamsUser", testutils.GetTeamsUserID()).Return(client, nil).Times(2)
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				p.On("GetBotUserID").Return(testutils.GetSenderID()).Times(2)
				p.On("GetSyncDirectMessages").Return(true).Once()
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				client.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					LastUpdateAt:    msTeamsLastUpdateAtTime,
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetPost", "mockMattermostID").Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetSenderID(), time.Now().UnixMicro()), nil).Times(1)
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
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetTeamsUserID(), nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMSTeams, true).Times(1)
			},
		},
		{
			description: "Valid: sync linked channels disabled",
			activityIds: clientmodels.ActivityIds{
				TeamID:    "mockTeamID",
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncLinkedChannels").Return(false).Once()
				p.On("GetSyncReactions").Return(true).Maybe()
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetMessage", "mockTeamID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					TeamID:          "mockTeamID",
					ChannelID:       testutils.GetChannelID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					LastUpdateAt:    msTeamsLastUpdateAtTime,
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetPost", "mockMattermostID").Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetSenderID(), time.Now().UnixMicro()), nil).Times(1)
				mockAPI.On("GetReactions", "mockMattermostID").Return([]*model.Reaction{}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("MattermostToTeamsUserID", "mock-BotUserID").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MSTeamsLastUpdateAt: time.Now(),
					MattermostID:        "mockMattermostID",
				}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
			},
		},
		{
			description: "Valid: channel message",
			activityIds: clientmodels.ActivityIds{
				TeamID:    "mockTeamID",
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, client *mocksClient.Client, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncLinkedChannels").Return(true).Once()
				p.On("GetSyncReactions").Return(true).Maybe()
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetBotUserID").Return("mock-BotUserID").Times(1)
				p.On("GetBotUserID").Return(testutils.GetSenderID()).Times(2)
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetMessage", "mockTeamID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					TeamID:          "mockTeamID",
					ChannelID:       testutils.GetChannelID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					LastUpdateAt:    msTeamsLastUpdateAtTime,
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetPost", "mockMattermostID").Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetSenderID(), time.Now().UnixMicro()), nil).Times(1)
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
					Creator:             "mockCreator",
					MattermostChannelID: testutils.GetChannelID(),
				}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMSTeams, false).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			ah := ActivityHandler{}
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			mockmetrics := mocksMetrics.NewMetrics(t)
			testCase.setupPlugin(p, client, mockAPI, store, mockmetrics)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testutils.MockLogs(mockAPI)
			testCase.setupStore(store)
			testCase.setupMetrics(mockmetrics)
			subscriptionID := "test"

			ah.plugin = p
			discardedReason := ah.handleUpdatedActivity(nil, subscriptionID, testCase.activityIds)

			lastSubscriptionActivity, ok := ah.lastUpdateAtMap.Load(subscriptionID)
			if discardedReason == "" {
				assert.True(t, ok)
				assert.Equal(t, lastSubscriptionActivity, msTeamsLastUpdateAtTime)
			} else {
				assert.False(t, ok)
			}
		})
	}
}

func TestHandleDeletedActivity(t *testing.T) {
	for _, testCase := range []struct {
		description  string
		activityIds  clientmodels.ActivityIds
		setupPlugin  func(*mocksPlugin.PluginIface, *plugintest.API, *mocksStore.Store, *mocksMetrics.Metrics)
		setupAPI     func(*plugintest.API)
		setupStore   func(*mocksStore.Store)
		setupMetrics func(*mocksMetrics.Metrics)
	}{
		{
			description: "Successfully deleted post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetStore").Return(store).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetMetrics").Return(mockmetrics).Maybe()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("DeletePost", testutils.GetMattermostID()).Return(nil).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("GetPostInfoByMSTeamsID", fmt.Sprintf("%s%s", testutils.GetChatID(), testutils.GetChannelID()), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetMattermostID(),
				}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionDeleted, metrics.ActionSourceMSTeams, true).Times(1)
			},
		},
		{
			description: "Unable to get post info by MS teams ID",
			activityIds: clientmodels.ActivityIds{
				ChannelID: testutils.GetChannelID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetStore").Return(store).Maybe()
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), "").Return(nil, errors.New("Error while getting post info by MS teams ID")).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Unable to to delete post",
			activityIds: clientmodels.ActivityIds{
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetStore").Return(store).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("DeletePost", "").Return(&model.AppError{
					Message: "Error while deleting a post",
				}).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			ah := ActivityHandler{}
			mockAPI := &plugintest.API{}
			store := mocksStore.NewStore(t)
			mockmetrics := mocksMetrics.NewMetrics(t)
			ah.plugin = p

			testCase.setupPlugin(p, mockAPI, store, mockmetrics)
			testCase.setupAPI(mockAPI)
			testutils.MockLogs(mockAPI)
			testCase.setupStore(store)
			testCase.setupMetrics(mockmetrics)

			ah.handleDeletedActivity(testCase.activityIds)
		})
	}
}

func TestHandleReactions(t *testing.T) {
	for _, testCase := range []struct {
		description  string
		reactions    []clientmodels.Reaction
		setupPlugin  func(*mocksPlugin.PluginIface, *plugintest.API, *mocksStore.Store, *mocksMetrics.Metrics)
		setupAPI     func(*plugintest.API)
		setupStore   func(*mocksStore.Store)
		setupMetrics func(*mocksMetrics.Metrics)
	}{
		{
			description: "Disabled by configuration",
			reactions:   []clientmodels.Reaction{},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(false).Once()
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Reactions list is empty",
			reactions:   []clientmodels.Reaction{},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(true).Once()
				p.On("GetAPI").Return(mockAPI).Maybe()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetReactions", testutils.GetPostID()).Return([]*model.Reaction{}, nil).Times(1)
			},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Unable to get the reactions",
			reactions: []clientmodels.Reaction{
				{
					UserID:   testutils.GetTeamsUserID(),
					Reaction: "+1",
				},
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(true).Once()
				p.On("GetAPI").Return(mockAPI).Maybe()
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetReactions", testutils.GetPostID()).Return(nil, testutils.GetInternalServerAppError("unable to get the reaction")).Times(1)
			},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Unable to find the user for the reaction",
			reactions: []clientmodels.Reaction{
				{
					UserID:   testutils.GetTeamsUserID(),
					Reaction: "+1",
				},
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(true).Once()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetMetrics").Return(mockmetrics).Maybe()
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
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return("", errors.New("unable to find the user for the reaction")).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMSTeams, false).Times(1)
			},
		},
		{
			description: "Unable to remove the reaction",
			reactions: []clientmodels.Reaction{
				{
					UserID:   testutils.GetTeamsUserID(),
					Reaction: "+1",
				},
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetSyncReactions").Return(true).Once()
				p.On("GetStore").Return(store).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetMetrics").Return(mockmetrics).Maybe()
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

				mockAPI.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetID(), nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMSTeams, false).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			ah := ActivityHandler{}
			mockAPI := &plugintest.API{}
			store := mocksStore.NewStore(t)
			mockmetrics := mocksMetrics.NewMetrics(t)
			ah.plugin = p

			testCase.setupPlugin(p, mockAPI, store, mockmetrics)
			testCase.setupAPI(mockAPI)
			testutils.MockLogs(mockAPI)
			testCase.setupStore(store)
			testCase.setupMetrics(mockmetrics)

			ah.handleReactions(testutils.GetPostID(), testutils.GetChannelID(), false, testCase.reactions)
		})
	}
}
