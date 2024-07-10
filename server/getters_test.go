package main

import (
	"errors"
	"testing"

	metricsmocks "github.com/mattermost/mattermost-plugin-msteams/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
