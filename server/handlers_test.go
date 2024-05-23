package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	metricsmocks "github.com/mattermost/mattermost-plugin-msteams/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCreatedActivity(t *testing.T) {
	msteamsCreateAtTime := time.Now()
	mmCreateAtTime := msteamsCreateAtTime.UnixNano() / int64(time.Millisecond)
	for _, testCase := range []struct {
		description  string
		activityIds  clientmodels.ActivityIds
		setupClient  func(*clientmocks.Client, *clientmocks.Client)
		setupAPI     func(*plugintest.API)
		setupStore   func(*storemocks.Store)
		setupMetrics func(*metricsmocks.Metrics)
	}{
		{
			description: "Unable to get original message",
			activityIds: clientmodels.ActivityIds{
				ChatID: "invalid-ChatID",
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", "invalid-ChatID").Return(nil, errors.New("Error while getting original chat")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Message is nil",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return("mock-mmUserID", nil)
				store.On("GetTokenForMattermostUser", "mock-mmUserID").Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Skipping not user event",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Duplicate post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
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
				mockAPI.On("GetPost", "mockMattermostID").Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetSenderID(), time.Now().UnixMicro()), nil).Times(1)
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{MattermostID: "mockMattermostID"}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Skipping messages from bot user",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
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
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Unable to get channel ID",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
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
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Channel ID is empty",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
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
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", "mockUserID-1").Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Unable to create post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
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
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(2)
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)

				store.On("GetTokenForMattermostUser", "mockUserID-1").Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Error updating the post link table",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
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
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:        testutils.GetID(),
					MSTeamsID:           testutils.GetMessageID(),
					MSTeamsChannel:      testutils.GetMSTeamsChannelID(),
					MSTeamsLastUpdateAt: msteamsCreateAtTime,
				}).Return(errors.New("unable to update the post")).Times(1)
				store.On("SetUsersLastChatReceivedAt", []string{"mockUserID-1", "mockUserID-2"}, storemodels.MilliToMicroSeconds(mmCreateAtTime)).Return(nil)
				store.On("GetTokenForMattermostUser", "mockUserID-1").Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMSTeams, true).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMSTeams, true, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Message generated from Mattermost",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText<abbr title=\"generated-from-mattermost\"></abbr>",
					CreateAt:        msteamsCreateAtTime,
					LastUpdateAt:    msteamsCreateAtTime,
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetDirectChannel", "mockUserID-1", "mockUserID-2").Return(&model.Channel{Id: testutils.GetChannelID()}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("CreatePost", testutils.GetPostFromTeamsMessage(mmCreateAtTime)).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), mmCreateAtTime), nil).Times(1)
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Valid: chat message",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{UserID: "mockUserID-1"},
						{UserID: "mockUserID-2"},
					},
					Type: "D",
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
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
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-1").Return("mockUserID-1", nil).Times(1)
				store.On("TeamsToMattermostUserID", "mockUserID-2").Return("mockUserID-2", nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:        testutils.GetID(),
					MSTeamsID:           testutils.GetMessageID(),
					MSTeamsChannel:      testutils.GetMSTeamsChannelID(),
					MSTeamsLastUpdateAt: msteamsCreateAtTime,
				}).Return(nil).Times(1)
				store.On("SetUsersLastChatReceivedAt", []string{"mockUserID-1", "mockUserID-2"}, storemodels.MilliToMicroSeconds(mmCreateAtTime)).Return(nil)
				store.On("GetTokenForMattermostUser", "mockUserID-1").Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMSTeams, true).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMSTeams, true, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Valid: sync linked channels disabled",
			activityIds: clientmodels.ActivityIds{
				TeamID:    "mockTeamID",
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
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
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
			},
		},
		{
			description: "Valid: channel message",
			activityIds: clientmodels.ActivityIds{
				TeamID:    "mockTeamID",
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
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
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetLinkByMSTeamsChannelID", "mockTeamID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					Creator:             "mockCreator",
					MattermostChannelID: testutils.GetChannelID(),
				}, nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(nil, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:        testutils.GetID(),
					MSTeamsID:           testutils.GetMessageID(),
					MSTeamsChannel:      testutils.GetChannelID(),
					MSTeamsLastUpdateAt: msteamsCreateAtTime,
				}).Return(nil).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMSTeams, false).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMSTeams, false, mock.AnythingOfType("time.Duration")).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			ah := ActivityHandler{}

			testCase.setupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			testCase.setupStore(p.store.(*storemocks.Store))
			testCase.setupAPI(p.API.(*plugintest.API))
			testCase.setupMetrics(p.metricsService.(*metricsmocks.Metrics))
			testutils.MockLogs(p.API.(*plugintest.API))

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
		setupClient  func(*clientmocks.Client, *clientmocks.Client)
		setupAPI     func(*plugintest.API)
		setupStore   func(*storemocks.Store)
		setupMetrics func(*metricsmocks.Metrics)
	}{
		{
			description: "Unable to get original message",
			activityIds: clientmodels.ActivityIds{
				ChatID: "invalid-ChatID",
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", "invalid-ChatID").Return(nil, errors.New("error while getting original chat")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *storemocks.Store) {},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {},
		},
		{
			description: "Message is nil",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Skipping not user event",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{}, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Skipping messages from bot user",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetTeamsUserID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Unable to get the post info",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Unable to get the post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
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
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID:        "mockMattermostID",
					MSTeamsID:           "mockMSTeamsID",
					MSTeamsChannel:      "mockMSTeamsChannel",
					MSTeamsLastUpdateAt: time.Now(),
				}, nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Unable to get and recover the post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
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
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID:        "mockMattermostID",
					MSTeamsID:           "mockMSTeamsID",
					MSTeamsChannel:      "mockMSTeamsChannel",
					MSTeamsLastUpdateAt: time.Now(),
				}, nil).Times(1)
				store.On("RecoverPost", "mockMattermostID").Return(errors.New("unable to recover"))
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Valid: chat message",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
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
				mockAPI.On("GetFileInfo", mock.Anything).Return(testutils.GetFileInfo(), nil).Once()
			},
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID:        "mockMattermostID",
					MSTeamsID:           "mockMSTeamsID",
					MSTeamsChannel:      "mockMSTeamsChannel",
					MSTeamsLastUpdateAt: time.Now(),
				}, nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMSTeams, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Valid: sync linked channels disabled",
			activityIds: clientmodels.ActivityIds{
				TeamID:    "mockTeamID",
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
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
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MSTeamsLastUpdateAt: time.Now(),
					MattermostID:        "mockMattermostID",
				}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
			},
		},
		{
			description: "Valid: channel message",
			activityIds: clientmodels.ActivityIds{
				TeamID:    "mockTeamID",
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
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
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
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
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMSTeams, false).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)

			testCase.setupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			testCase.setupStore(p.store.(*storemocks.Store))
			testCase.setupAPI(p.API.(*plugintest.API))
			testCase.setupMetrics(p.metricsService.(*metricsmocks.Metrics))
			testutils.MockLogs(p.API.(*plugintest.API))
			subscriptionID := "test"

			ah := ActivityHandler{}
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
		setupAPI     func(*plugintest.API)
		setupStore   func(*storemocks.Store)
		setupMetrics func(*metricsmocks.Metrics)
	}{
		{
			description: "Successfully deleted post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("DeletePost", testutils.GetMattermostID()).Return(nil).Times(1)
			},
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", fmt.Sprintf("%s%s", testutils.GetChatID(), testutils.GetChannelID()), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetMattermostID(),
				}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionDeleted, metrics.ActionSourceMSTeams, true).Times(1)
			},
		},
		{
			description: "Unable to get post info by MS teams ID",
			activityIds: clientmodels.ActivityIds{
				ChannelID: testutils.GetChannelID(),
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), "").Return(nil, errors.New("Error while getting post info by MS teams ID")).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {},
		},
		{
			description: "Unable to to delete post",
			activityIds: clientmodels.ActivityIds{
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("DeletePost", "").Return(&model.AppError{
					Message: "Error while deleting a post",
				}).Times(1)
			},
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			testCase.setupStore(p.store.(*storemocks.Store))
			testCase.setupAPI(p.API.(*plugintest.API))
			testCase.setupMetrics(p.metricsService.(*metricsmocks.Metrics))
			testutils.MockLogs(p.API.(*plugintest.API))
			ah := ActivityHandler{}
			ah.plugin = p

			ah.handleDeletedActivity(testCase.activityIds)
		})
	}
}

func TestHandleReactions(t *testing.T) {
	for _, testCase := range []struct {
		description  string
		reactions    []clientmodels.Reaction
		setupAPI     func(*plugintest.API)
		setupStore   func(*storemocks.Store)
		setupMetrics func(*metricsmocks.Metrics)
	}{
		{
			description: "Disabled by configuration",
			reactions:   []clientmodels.Reaction{},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *storemocks.Store) {},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {},
		},
		{
			description: "Reactions list is empty",
			reactions:   []clientmodels.Reaction{},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetReactions", testutils.GetPostID()).Return([]*model.Reaction{}, nil).Times(1)
			},
			setupStore:   func(store *storemocks.Store) {},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {},
		},
		{
			description: "Unable to get the reactions",
			reactions: []clientmodels.Reaction{
				{
					UserID:   testutils.GetTeamsUserID(),
					Reaction: "+1",
				},
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetReactions", testutils.GetPostID()).Return(nil, testutils.GetInternalServerAppError("unable to get the reaction")).Times(1)
			},
			setupStore:   func(store *storemocks.Store) {},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {},
		},
		{
			description: "Unable to find the user for the reaction",
			reactions: []clientmodels.Reaction{
				{
					UserID:   testutils.GetTeamsUserID(),
					Reaction: "+1",
				},
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
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return("", errors.New("unable to find the user for the reaction")).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
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
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetID(), nil).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMSTeams, false).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			testCase.setupStore(p.store.(*storemocks.Store))
			testCase.setupAPI(p.API.(*plugintest.API))
			testCase.setupMetrics(p.metricsService.(*metricsmocks.Metrics))
			testutils.MockLogs(p.API.(*plugintest.API))
			ah := ActivityHandler{}
			ah.plugin = p

			ah.handleReactions(testutils.GetPostID(), testutils.GetChannelID(), false, testCase.reactions)
		})
	}
}

func TestShouldSyncChat(t *testing.T) {
	testCases := []struct {
		Name             string
		ChatMembersCount int

		EnableDM bool
		EnableGM bool

		ShouldSync          bool
		ReasonForNotSyncing string
	}{
		{
			Name:             "should sync self messages if DM are enabled",
			ChatMembersCount: 1,
			EnableDM:         true,
			ShouldSync:       true,
		},
		{
			Name:                "should not sync self messages if DM are disabled",
			ChatMembersCount:    1,
			EnableDM:            false,
			ShouldSync:          false,
			ReasonForNotSyncing: metrics.DiscardedReasonDirectMessagesDisabled,
		},
		{
			Name:             "should sync DMs if DM are enabled",
			ChatMembersCount: 2,
			EnableDM:         true,
			ShouldSync:       true,
		},
		{
			Name:                "should not sync DMs if DM are disabled",
			ChatMembersCount:    2,
			EnableDM:            false,
			ShouldSync:          false,
			ReasonForNotSyncing: metrics.DiscardedReasonDirectMessagesDisabled,
		},
		{
			Name:             "should sync GMs if GM are enabled",
			ChatMembersCount: 3,
			EnableGM:         true,
			ShouldSync:       true,
		},
		{
			Name:                "should not sync GMs if GM are disabled",
			ChatMembersCount:    3,
			EnableGM:            false,
			ShouldSync:          false,
			ReasonForNotSyncing: metrics.DiscardedReasonGroupMessagesDisabled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			assert := require.New(t)
			p := newTestPlugin(t)
			configuration := p.getConfiguration().Clone()
			configuration.SyncDirectMessages = tc.EnableDM
			configuration.SyncGroupMessages = tc.EnableGM
			p.setConfiguration(configuration)

			ah := ActivityHandler{
				plugin: p,
			}

			chat := &clientmodels.Chat{
				Members: make([]clientmodels.ChatMember, tc.ChatMembersCount),
			}

			shouldSync, reason := ah.ShouldSyncDMGMChannel(chat)
			assert.Equal(tc.ShouldSync, shouldSync)
			if !tc.ShouldSync {
				assert.Equal(tc.ReasonForNotSyncing, reason)
			}
		})
	}
}
