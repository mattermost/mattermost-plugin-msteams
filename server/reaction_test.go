package main

import (
	"math"
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
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSetChatReaction(t *testing.T) {
	mockChat := &clientmodels.Chat{
		ID: testutils.GetChatID(),
		Members: []clientmodels.ChatMember{
			{
				DisplayName: "mockDisplayName",
				UserID:      testutils.GetTeamsUserID(),
				Email:       testutils.GetTestEmail(),
			},
		},
	}
	mockChatMessage := &clientmodels.Message{
		ID:           "mockTeamsMessageID",
		ChatID:       testutils.GetChatID(),
		LastUpdateAt: testutils.GetMockTime(),
	}
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
		UpdateRequired  bool
	}{
		{
			Name:     "SetChatReaction: Unable to get the source user ID",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", testutils.GetInternalServerAppError("unable to get the source user ID")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the source user ID",
		},
		{
			Name:     "SetChatReaction: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "not connected user",
		},
		{
			Name: "SetChatReaction: Unable to get the chat ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the channel",
		},
		{
			Name: "SetChatReaction: Unable to set the chat reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("SetChatReaction", testutils.GetChatID(), "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(nil, errors.New("unable to set the chat reaction")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.SetChatReaction", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "unable to set the chat reaction",
			UpdateRequired:  true,
		},
		{
			Name: "SetChatReaction: Update not required on MS Teams",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", testutils.GetMockTime()).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), "mockTeamsMessageID").Return(mockChatMessage, nil).Once()
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "SetChatReaction: Unable to set the post last updateAt time",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", testutils.GetMockTime()).Return(errors.New("unable to set post lastUpdateAt value")).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("SetChatReaction", testutils.GetChatID(), "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionSetAction, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SetChatReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
		{
			Name: "SetChatReaction: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", testutils.GetMockTime()).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("SetChatReaction", testutils.GetChatID(), "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionSetAction, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SetChatReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			resp := p.SetChatReaction("mockTeamsMessageID", testutils.GetID(), testutils.GetChannelID(), "mockEmojiName", test.UpdateRequired)
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}

func TestSetReaction(t *testing.T) {
	mockChannelMessage := &clientmodels.Message{
		ID:           testutils.GetID(),
		TeamID:       "mockTeamsTeamID",
		ChannelID:    "mockTeamsChannelID",
		LastUpdateAt: testutils.GetMockTime(),
	}
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
	}{
		{
			Name:     "SetReaction: Unable to get the post info",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the post info",
		},
		{
			Name:     "SetReaction: Post info is nil",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "teams message not found",
		},
		{
			Name:     "SetReaction: Unable to get the client",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", mock.Anything).Return(nil, nil).Times(2)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "not connected user",
		},
		{
			Name: "SetReaction: Unable to set the reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(nil, errors.New("unable to set the reaction")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.SetReaction", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "unable to set the reaction",
		},
		{
			Name: "SetReaction: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", "", testutils.GetMockTime()).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionSetAction, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SetReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))

			resp := p.SetReaction("mockTeamsTeamID", "mockTeamsChannelID", testutils.GetUserID(), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), "mockName", true)
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}

func TestUnsetChatReaction(t *testing.T) {
	mockChat := &clientmodels.Chat{
		ID: testutils.GetChatID(),
		Members: []clientmodels.ChatMember{
			{
				DisplayName: "mockDisplayName",
				UserID:      testutils.GetTeamsUserID(),
				Email:       testutils.GetTestEmail(),
			},
		},
	}
	mockChatMessage := &clientmodels.Message{
		ID:           "mockTeamsMessageID",
		ChatID:       testutils.GetChatID(),
		LastUpdateAt: testutils.GetMockTime(),
	}
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
	}{
		{
			Name:     "UnsetChatReaction: Unable to get the source user ID",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", testutils.GetInternalServerAppError("unable to get the source user ID")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the source user ID",
		},
		{
			Name:     "UnsetChatReaction: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "not connected user",
		},
		{
			Name: "UnsetChatReaction: Unable to get the chat ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the channel",
		},
		{
			Name: "UnsetChatReaction: Unable to unset the chat reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("UnsetChatReaction", testutils.GetChatID(), "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(nil, errors.New("unable to unset the chat reaction")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetChatReaction", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "unable to unset the chat reaction",
		},
		{
			Name: "UnsetChatReaction: Unable to set the post last updateAt time",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", testutils.GetMockTime()).Return(errors.New("unable to set post lastUpdateAt value")).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("UnsetChatReaction", testutils.GetChatID(), "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetChatReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "UnsetChatReaction: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", testutils.GetMockTime()).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("UnsetChatReaction", testutils.GetChatID(), "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetChatReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			resp := p.UnsetChatReaction("mockTeamsMessageID", testutils.GetID(), testutils.GetChannelID(), "mockEmojiName")
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}

func TestUnsetReaction(t *testing.T) {
	mockChannelMessage := &clientmodels.Message{
		ID:           testutils.GetID(),
		TeamID:       "mockTeamsTeamID",
		ChannelID:    "mockTeamsChannelID",
		LastUpdateAt: testutils.GetMockTime(),
	}
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
	}{
		{
			Name:     "UnsetReaction: Unable to get the post info",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the post info",
		},
		{
			Name:     "UnsetReaction: Post info is nil",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "teams message not found",
		},
		{
			Name:     "UnsetReaction: Unable to get the client",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", mock.Anything).Return(nil, nil).Times(2)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "not connected user",
		},
		{
			Name: "UnsetReaction: Unable to unset the reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(nil, errors.New("unable to unset the reaction")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetReaction", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "unable to unset the reaction",
		},
		{
			Name: "UnsetReaction: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", "", testutils.GetMockTime()).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))

			resp := p.UnsetReaction("mockTeamsTeamID", "mockTeamsChannelID", testutils.GetUserID(), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), "mockName")
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}
