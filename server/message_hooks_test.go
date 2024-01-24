package main

import (
	"bytes"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	metricsmocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestReactionHasBeenAdded(t *testing.T) {
	mockMessage := &clientmodels.Message{
		ID:           "ms-teams-id",
		TeamID:       "ms-teams-team-id",
		ChannelID:    "ms-teams-channel-id",
		LastUpdateAt: testutils.GetMockTime(),
	}
	for _, test := range []struct {
		Name         string
		SetupAPI     func(*plugintest.API)
		SetupStore   func(*storemocks.Store)
		SetupClient  func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics func(*metricsmocks.Metrics)
	}{
		{
			Name:     "ReactionHasBeenAdded: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client, uc *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessageHooksEvent", "reaction_added").Times(1)
			},
		},
		{
			Name: "ReactionHasBeenAdded: Unable to get the link by channel ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("LogWarn", "Unable to handle message reaction set", "error", mock.Anything).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("MattermostToTeamsUserID", mock.AnythingOfType("string")).Return("", testutils.GetInternalServerAppError("unable to get the source user ID")).Times(1)
				store.On("GetDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID()).Return(time.Now().Add(time.Hour*2).Add(time.Hour*2), nil).Once()
			},
			SetupClient: func(c *clientmocks.Client, uc *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessageHooksEvent", "reaction_added").Times(1)
			},
		},
		{
			Name: "ReactionHasBeenAdded: Unable to get the link by channel ID and channel",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client, uc *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessageHooksEvent", "reaction_added").Times(1)
			},
		},
		{
			Name: "ReactionHasBeenAdded: Unable to get the post",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post")).Times(1)
				api.On("LogError", "Unable to get the post from the reaction", "reaction", mock.Anything, "error", mock.Anything).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{}, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client, uc *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessageHooksEvent", "reaction_added").Times(1)
			},
		},
		{
			Name: "ReactionHasBeenAdded: Unable to set the reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogError", "Error setting reaction", "error", "unable to set the reaction")
				api.On("LogWarn", "Unable to handle message reaction set", "error", "unable to set the reaction")
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{MattermostID: testutils.GetID(), MSTeamsID: "ms-teams-id", MSTeamsChannel: "ms-teams-channel-id", MSTeamsLastUpdateAt: time.UnixMicro(100)}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{MattermostTeamID: "mm-team-id", MattermostChannelID: "mm-channel-id", MSTeamsTeam: "ms-teams-team-id", MSTeamsChannel: "ms-teams-channel-id"}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SetReaction", "ms-teams-team-id", "ms-teams-channel-id", "", "ms-teams-id", testutils.GetID(), mock.AnythingOfType("string")).Return(nil, errors.New("unable to set the reaction")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.SetReaction", "false", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMessageHooksEvent", "reaction_added").Times(1)
			},
		},
		{
			Name: "ReactionHasBeenAdded: Unable to set the post last updateAt time",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", "unable to set post lastUpdateAt value")
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{MattermostID: testutils.GetID(), MSTeamsID: "ms-teams-id", MSTeamsChannel: "ms-teams-channel-id", MSTeamsLastUpdateAt: time.UnixMicro(100)}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{MattermostTeamID: "mm-team-id", MattermostChannelID: "mm-channel-id", MSTeamsTeam: "ms-teams-team-id", MSTeamsChannel: "ms-teams-channel-id"}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", testutils.GetID(), testutils.GetMockTime()).Return(errors.New("unable to set post lastUpdateAt value")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SetReaction", "ms-teams-team-id", "ms-teams-channel-id", "", "ms-teams-id", testutils.GetID(), mock.AnythingOfType("string")).Return(mockMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionSetAction, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SetReaction", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "reaction_added").Times(1)
			},
		},
		{
			Name: "ReactionHasBeenAdded: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{MattermostID: testutils.GetID(), MSTeamsID: "ms-teams-id", MSTeamsChannel: "ms-teams-channel-id", MSTeamsLastUpdateAt: time.UnixMicro(100)}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{MattermostTeamID: "mm-team-id", MattermostChannelID: "mm-channel-id", MSTeamsTeam: "ms-teams-team-id", MSTeamsChannel: "ms-teams-channel-id"}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", testutils.GetID(), testutils.GetMockTime()).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SetReaction", "ms-teams-team-id", "ms-teams-channel-id", "", "ms-teams-id", testutils.GetID(), mock.AnythingOfType("string")).Return(mockMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionSetAction, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SetReaction", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "reaction_added").Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			p.configuration.SyncDirectMessages = true
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			p.ReactionHasBeenAdded(&plugin.Context{}, testutils.GetReaction())
		})
	}
}

func TestReactionHasBeenRemoved(t *testing.T) {
	mockMessage := &clientmodels.Message{
		ID:           "ms-teams-id",
		TeamID:       "ms-teams-team-id",
		ChannelID:    "ms-teams-channel-id",
		LastUpdateAt: testutils.GetMockTime(),
	}
	for _, test := range []struct {
		Name         string
		SetupAPI     func(*plugintest.API)
		SetupStore   func(*storemocks.Store)
		SetupClient  func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics func(*metricsmocks.Metrics)
	}{
		{
			Name:     "ReactionHasBeenRemoved: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessageHooksEvent", "reaction_removed").Times(1)
			},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to get the post",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get the post from the reaction", "reaction", mock.Anything, "error", "unable to get the post").Times(1)
				api.On("GetPost", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessageHooksEvent", "reaction_removed").Times(1)
			},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to get the link by channel ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Unable to handle chat message reaction unset", "error", "unable to get source user ID").Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(1)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", errors.New("unable to get source user ID")).Times(1)
				store.On("GetDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID()).Return(time.Now().Add(time.Hour*2), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessageHooksEvent", "reaction_removed").Times(1)
			},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to get the link by channel ID and channel",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(1)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessageHooksEvent", "reaction_removed").Times(1)
			},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to remove the reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error in removing the reaction", "emojiName", testutils.GetReaction().EmojiName, "error", "unable to unset the reaction").Times(1)
				api.On("LogWarn", "Unable to handle message reaction unset", "error", "unable to unset the reaction").Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MattermostTeamID:    "mockMattermostTeam",
					MattermostChannelID: "mockMattermostChannel",
					MSTeamsTeam:         "mockTeamsTeamID",
					MSTeamsChannel:      "mockTeamsChannelID",
				}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), mock.AnythingOfType("string")).Return(nil, errors.New("unable to unset the reaction")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetReaction", "false", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMessageHooksEvent", "reaction_removed").Times(1)
			},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to set the post last updateAt time",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", "unable to set post lastUpdateAt value")
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MattermostTeamID:    "mockMattermostTeam",
					MattermostChannelID: "mockMattermostChannel",
					MSTeamsTeam:         "mockTeamsTeamID",
					MSTeamsChannel:      "mockTeamsChannelID",
				}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", testutils.GetID(), testutils.GetMockTime()).Return(errors.New("unable to set post lastUpdateAt value")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), mock.AnythingOfType("string")).Return(mockMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetReaction", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "reaction_removed").Times(1)
			},
		},
		{
			Name: "ReactionHasBeenRemoved: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MattermostTeamID:    "mockMattermostTeam",
					MattermostChannelID: "mockMattermostChannel",
					MSTeamsTeam:         "mockTeamsTeamID",
					MSTeamsChannel:      "mockTeamsChannelID",
				}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", testutils.GetID(), testutils.GetMockTime()).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), mock.AnythingOfType("string")).Return(mockMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetReaction", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "reaction_removed").Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			p.configuration.SyncDirectMessages = true
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			p.ReactionHasBeenRemoved(&plugin.Context{}, testutils.GetReaction())
		})
	}
}

func TestMessageHasBeenUpdated(t *testing.T) {
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
	mockChannelMessage := &clientmodels.Message{
		ID:        "mockMessageID",
		TeamID:    "mockTeamsTeamID",
		ChannelID: "mockTeamsChannelID",
	}
	mockChatMessage := &clientmodels.Message{
		ID:     "ms-teams-id",
		ChatID: testutils.GetChatID(),
	}
	for _, test := range []struct {
		Name         string
		SetupAPI     func(*plugintest.API)
		SetupStore   func(*storemocks.Store)
		SetupClient  func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics func(*metricsmocks.Metrics)
	}{
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("mockChatID", nil).Times(2)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMsgID",
				}, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMsgID",
					MSTeamsChannel: testutils.GetChatID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("UpdateChatMessage", testutils.GetChatID(), "mockMsgID", "", []models.ChatMessageMentionable{}).Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateChatMessage", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "message_update").Times(1)
			},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and channel",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessageHooksEvent", "message_update").Times(1)
			},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and channel type is Open",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeOpen), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessageHooksEvent", "message_update").Times(1)
			},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and unable to get channel members",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(nil, testutils.GetInternalServerAppError("unable to get channel members")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessageHooksEvent", "message_update").Times(1)
			},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and unable to update the chat",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error getting post info", "error", mock.Anything).Times(1)
				api.On("LogError", "Unable to handle message update", "error", mock.Anything).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(2)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get post info")).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMessageHooksEvent", "message_update").Times(1)
			},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and unable to create or get chat for users",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("LogError", "Unable to create or get chat for users", "error", "unable to create or get chat for users").Return().Once()
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(2)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(nil, errors.New("unable to create or get chat for users")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "false", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMessageHooksEvent", "message_update").Times(1)
			},
		},
		{
			Name: "MessageHasBeenUpdated: Able to get the link by channel ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MattermostTeamID:    "mockMattermostTeam",
					MattermostChannelID: "mockMattermostChannel",
					MSTeamsTeam:         "mockTeamsTeamID",
					MSTeamsChannel:      "mockTeamsChannelID",
				}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMessageID",
				}, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: "mockTeamsChannelID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", "mockTeamsChannelID", "", "mockMessageID", "", []models.ChatMessageMentionable{}).Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "message_update").Times(1)
			},
		},
		{
			Name: "MessageHasBeenUpdated: Able to get the link by channel ID but unable to update post",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the post on MS Teams", "error", errors.New("unable to update the post")).Return(nil).Times(1)
				api.On("LogError", "Unable to handle message update", "error", mock.Anything).Return(nil).Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MattermostTeamID:    "mockMattermostTeamID",
					MattermostChannelID: "mockMattermostChannelID",
					MSTeamsTeam:         "mockTeamsTeamID",
					MSTeamsChannel:      "mockTeamsChannelID",
				}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockTeamsTeamID",
					MSTeamsChannel: "mockTeamsChannelID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", "mockTeamsChannelID", "", "", "", []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to update the post")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "false", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "message_update").Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			p.configuration.SyncDirectMessages = true
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			p.MessageHasBeenUpdated(&plugin.Context{}, testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()))
		})
	}
}

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
				store.On("GetDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID()).Return(time.Now().Add(time.Hour*2), nil).Once()
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
				store.On("GetDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID()).Return(time.Now().Add(time.Hour*2), nil).Once()
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "not connected user",
		},
		{
			Name: "SetChatReaction: Unable to get the chat ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
				api.On("LogError", "Unable to get MM channel", "channelID", testutils.GetChannelID(), "error", "unable to get the channel")
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
				api.On("LogError", "Error creating post reaction", "error", mock.Anything)
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
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.SetChatReaction", "false", mock.AnythingOfType("float64")).Once()
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
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "SetChatReaction: Unable to set the post last updateAt time",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", "unable to set post lastUpdateAt value")
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
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SetChatReaction", "true", mock.AnythingOfType("float64")).Once()
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
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SetChatReaction", "true", mock.AnythingOfType("float64")).Once()
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
				api.On("LogError", "Error setting reaction", "error", "unable to set the reaction")
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
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.SetReaction", "false", mock.AnythingOfType("float64")).Once()
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
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SetReaction", "true", mock.AnythingOfType("float64")).Once()
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
			p.API.(*plugintest.API).On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

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
				store.On("GetDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID()).Return(time.Now().Add(time.Hour*2), nil).Once()
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
				store.On("GetDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID()).Return(time.Now().Add(time.Hour*2), nil).Once()
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "not connected user",
		},
		{
			Name: "UnsetChatReaction: Unable to get the chat ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Failed to create or get the chat", "error", mock.Anything).Return()
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
				api.On("LogError", "Unable to get MM channel", "channelID", testutils.GetChannelID(), "error", "unable to get the channel")
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
				api.On("LogError", "Error in removing the chat reaction", "emojiName", "mockEmojiName", "error", "unable to unset the chat reaction")
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
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetChatReaction", "false", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "unable to unset the chat reaction",
		},
		{
			Name: "UnsetChatReaction: Unable to set the post last updateAt time",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", "unable to set post lastUpdateAt value")
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
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetChatReaction", "true", mock.AnythingOfType("float64")).Once()
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
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetChatReaction", "true", mock.AnythingOfType("float64")).Once()
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
				api.On("LogError", "Error in removing the reaction", "emojiName", "mockName", "error", "unable to unset the reaction")
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
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetReaction", "false", mock.AnythingOfType("float64")).Once()
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
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetReaction", "true", mock.AnythingOfType("float64")).Once()
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
			p.API.(*plugintest.API).On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			resp := p.UnsetReaction("mockTeamsTeamID", "mockTeamsChannelID", testutils.GetUserID(), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), "mockName")
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}

func TestSendChat(t *testing.T) {
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
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
		ExpectedError   string
	}{
		{
			Name:     "SendChat: Unable to get the source user ID",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", errors.New("unable to get the source user ID")).Times(1)
				store.On("GetDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID()).Return(time.Now().Add(time.Hour*2), errors.New("error in getting prompt from store")).Once()
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "unable to get the source user ID",
		},
		{
			Name: "SendChat: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "Your Mattermost account is not connected to MS Teams so your activity will not be relayed to users on MS Teams. You can connect your account using the `/msteams-sync connect` slash command.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID()).Return(time.Now(), nil).Once()
				store.On("StoreDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID(), mock.AnythingOfType("time.Time")).Return(errors.New("error in storing prompt")).Once()
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "SendChat: Unable to create or get the chat",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Failed to create or get the chat", "error", errors.New("unable to create or get the chat"))
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(nil, errors.New("unable to create or get the chat")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "false", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError: "unable to create or get the chat",
		},
		{
			Name: "SendChat: Unable to send the chat",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error creating post on MS Teams", "error", "unable to send the chat")
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), mockChat).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), []*clientmodels.Attachment{{
					ID: testutils.GetID(),
				}}, []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to send the chat")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "false", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "attachment_created").Times(1)
			},
			ExpectedError: "unable to send the chat",
		},
		{
			Name: "SendChat: Able to send the chat and not able to store the post",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", mock.Anything)
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(testutils.GetInternalServerAppError("unable to store the post")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), []*clientmodels.Attachment{{
					ID: testutils.GetID(),
				}}, []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), mockChat).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", true).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "attachment_created").Times(1)
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "SendChat: Unable to get the parent message",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error in getting parent chat message", "error", errors.New("error in getting parent chat message"))
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(&storemodels.PostInfo{
					MSTeamsID: "mockParentMessageID",
				}, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Once()
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Once()
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), []*clientmodels.Attachment{{
					ID: testutils.GetID(),
				}}, []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), mockChat).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), "mockParentMessageID").Return(nil, errors.New("error in getting parent chat message")).Once()
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", true).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "false", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "attachment_created").Times(1)
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "SendChat: Unable to get the file info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Unable to get file info", "error", mock.Anything)
				api.On("GetFileInfo", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get file attachment")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, true).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "attachment_created").Times(1)
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "SendChat: Unable to get the file attachment from Mattermost",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error in getting file attachment from Mattermost", "error", mock.Anything)
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the file attachment from Mattermost")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, true).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "attachment_created").Times(1)
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "SendChat: Unable to upload the attachments",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error in uploading file attachment to MS Teams", "error", errors.New("unable to upload the attachments"))
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(&storemodels.PostInfo{
					MSTeamsID: "mockParentMessageID",
				}, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Once()
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Once()
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), mockChat).Return(nil, errors.New("unable to upload the attachments")).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), "mockParentMessageID").Return(&clientmodels.Message{
					ID:              "mockParentMessageID",
					UserID:          "mockUserID",
					Text:            "mockText",
					UserDisplayName: "mockUserDisplayName",
				}, nil).Once()
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", &clientmodels.Message{
					ID:              "mockParentMessageID",
					UserID:          "mockUserID",
					Text:            "mockText",
					UserDisplayName: "mockUserDisplayName",
				}, ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToUploadFileOnTeams, true).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "false", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "attachment_created").Times(1)
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "SendChat: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), mockChat).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), []*clientmodels.Attachment{{
					ID: testutils.GetID(),
				}}, []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", true).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "attachment_created").Times(1)
			},
			ExpectedMessage: "mockMessageID",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())
			mockPost.Message = "mockMessage??????????"
			mockPost.RootId = "mockRootID"
			resp, err := p.SendChat(testutils.GetID(), []string{testutils.GetID(), testutils.GetID()}, mockPost)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}

			assert.Equal(resp, test.ExpectedMessage)
		})
	}
}

func TestSend(t *testing.T) {
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
		ExpectedError   string
	}{
		{
			Name:     "Send: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "Send: Unable to get the file info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Unable to get file info", "error", mock.Anything).Times(1)
				api.On("GetFileInfo", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get file attachment")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n", ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "Send: Unable to get file attachment from Mattermost",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error in getting file attachment from Mattermost", "error", mock.Anything).Times(1)
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the file attachment from Mattermost")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n", ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "Send: Unable to send message with attachments",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error creating post on MS Teams", "error", "unable to send message with attachments").Times(1)
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UploadFile", testutils.GetID(), testutils.GetChannelID(), "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), (*clientmodels.Chat)(nil)).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n", []*clientmodels.Attachment{
					{
						ID: testutils.GetID(),
					},
				}, []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to send message with attachments")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "false", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "attachment_created").Times(1)
			},
			ExpectedError: "unable to send message with attachments",
		},
		{
			Name: "Send: Able to send message with attachments but unable to store posts",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", mock.Anything).Times(1)
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(errors.New("unable to store posts")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UploadFile", testutils.GetID(), testutils.GetChannelID(), "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), (*clientmodels.Chat)(nil)).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n", []*clientmodels.Attachment{
					{
						ID: testutils.GetID(),
					},
				}, []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "attachment_created").Times(1)
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "Send: Able to send message with attachments with no error",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UploadFile", testutils.GetID(), testutils.GetChannelID(), "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), (*clientmodels.Chat)(nil)).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n", []*clientmodels.Attachment{
					{
						ID: testutils.GetID(),
					},
				}, []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMessageHooksEvent", "attachment_created").Times(1)
			},
			ExpectedMessage: "mockMessageID",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())
			mockPost.Message = "mockMessage??????????"
			resp, err := p.Send(testutils.GetID(), testutils.GetChannelID(), testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), mockPost)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}

			assert.Equal(resp, test.ExpectedMessage)
		})
	}
}

func TestDelete(t *testing.T) {
	for _, test := range []struct {
		Name          string
		SetupAPI      func(*plugintest.API)
		SetupStore    func(*storemocks.Store)
		SetupClient   func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics  func(mockmetrics *metricsmocks.Metrics)
		ExpectedError string
	}{
		{
			Name:     "Delete: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "Delete: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error getting post info", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(metrics *metricsmocks.Metrics) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "Delete: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error deleting post, post not found.")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(metrics *metricsmocks.Metrics) {},
			ExpectedError: "post not found",
		},
		{
			Name: "Delete: Unable to delete the message",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error deleting post from MS Teams", "error", errors.New("unable to delete the message"))
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("DeleteMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID").Return(errors.New("unable to delete the message")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteMessage", "false", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError: "unable to delete the message",
		},
		{
			Name:     "Delete: Valid",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("DeleteMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID").Return(nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionDeleted, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteMessage", "true", mock.AnythingOfType("float64")).Once()
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
			err := p.Delete("mockTeamsTeamID", testutils.GetChannelID(), testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()))
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestDeleteChat(t *testing.T) {
	for _, test := range []struct {
		Name          string
		SetupAPI      func(*plugintest.API)
		SetupStore    func(*storemocks.Store)
		SetupClient   func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics  func(mockmetrics *metricsmocks.Metrics)
		ExpectedError string
	}{
		{
			Name:     "DeleteChat: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
				store.On("GetDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID()).Return(time.Now().Add(time.Hour*2), nil).Once()
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "DeleteChat: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error getting post info", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "DeleteChat: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error deleting post, post not found.")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "post not found",
		},
		{
			Name: "DeleteChat: Unable to delete the message",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error deleting post from MS Teams", "error", errors.New("unable to delete the message"))
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("DeleteChatMessage", "mockChatID", "mockMSTeamsID").Return(errors.New("unable to delete the message")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "false", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError: "unable to delete the message",
		},
		{
			Name:     "DeleteChat: Valid",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("DeleteChatMessage", "mockChatID", "mockMSTeamsID").Return(nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionDeleted, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "true", mock.AnythingOfType("float64")).Once()
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
			err := p.DeleteChat("mockChatID", testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()))
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	mockChannelMessage := &clientmodels.Message{
		ID:        "mockMSTeamsID",
		TeamID:    "mockTeamsTeamID",
		ChannelID: testutils.GetChannelID(),
	}
	for _, test := range []struct {
		Name           string
		SetupAPI       func(*plugintest.API)
		SetupStore     func(*storemocks.Store)
		SetupClient    func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics   func(mockmetrics *metricsmocks.Metrics)
		ExpectedError  string
		UpdateRequired bool
	}{
		{
			Name:     "Update: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "Update: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error getting post info", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "Update: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error updating post, post not found.")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "post not found",
		},
		{
			Name: "Update: Unable to update the message",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the post on MS Teams", "error", errors.New("unable to update the message"))
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to update the message")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "false", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError:  "unable to update the message",
			UpdateRequired: true,
		},
		{
			Name: "Update: Update not required on MS Teams",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error in getting the message from MS Teams", "error", errors.New("unable to get the updated message"))
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChannelID(),
					MSTeamsID:      "mockMSTeamsID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("GetMessage", "mockTeamsTeamID", testutils.GetChannelID(), "mockMSTeamsID").Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetMessage", "true", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "Update: Unable to store the link posts",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", mock.Anything)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChannelID(),
					MSTeamsID:      "mockMSTeamsID",
				}).Return(errors.New("unable to store the link posts")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "true", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
		{
			Name: "Update: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChannelID(),
					MSTeamsID:      "mockMSTeamsID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "true", mock.AnythingOfType("float64")).Once()
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
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())
			mockPost.Message = "mockMessage??????????"
			err := p.Update("mockTeamsTeamID", testutils.GetChannelID(), testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), mockPost, test.UpdateRequired)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestUpdateChat(t *testing.T) {
	mockChatMessage := &clientmodels.Message{
		ID:     "mockChatID",
		ChatID: "mockTeamsTeamID",
	}
	for _, test := range []struct {
		Name           string
		SetupAPI       func(*plugintest.API)
		SetupStore     func(*storemocks.Store)
		SetupClient    func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics   func(mockmetrics *metricsmocks.Metrics)
		ExpectedError  string
		UpdateRequired bool
	}{
		{
			Name: "UpdateChat: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error getting post info", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "UpdateChat: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error updating post, post not found.")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "post not found",
		},
		{
			Name:     "UpdateChat: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
				store.On("GetDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID()).Return(time.Now().Add(time.Hour*2), nil).Once()
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "UpdateChat: Unable to update the message",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the post on MS Teams", "error", errors.New("unable to update the message"))
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to update the message")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateChatMessage", "false", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError:  "unable to update the message",
			UpdateRequired: true,
		},
		{
			Name:     "UpdateChat: Update not required on MS Teams",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockTeamsTeamID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("GetChatMessage", "mockChatID", "mockTeamsTeamID").Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "UpdateChat: Unable to store the link posts",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockTeamsTeamID",
				}).Return(errors.New("unable to store the link posts")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateChatMessage", "true", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
		{
			Name: "UpdateChat: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockTeamsTeamID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateChatMessage", "true", mock.AnythingOfType("float64")).Once()
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
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())
			mockPost.Message = "mockMessage??????????"
			err := p.UpdateChat("mockChatID", testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), mockPost, test.UpdateRequired)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestGetChatIDForChannel(t *testing.T) {
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
	for _, test := range []struct {
		Name           string
		SetupAPI       func(*plugintest.API)
		SetupStore     func(*storemocks.Store)
		SetupClient    func(*clientmocks.Client)
		ExpectedError  string
		ExpectedResult string
	}{
		{
			Name: "GetChatIDForChannel: Unable to get the channel",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
				api.On("LogError", "Unable to get MM channel", "channelID", testutils.GetChannelID(), "error", "unable to get the channel")
			},
			SetupStore:    func(store *storemocks.Store) {},
			SetupClient:   func(client *clientmocks.Client) {},
			ExpectedError: "unable to get the channel",
		},
		{
			Name: "GetChatIDForChannel: Channel type is 'open'",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeOpen), nil).Times(1)
			},
			SetupStore:    func(store *storemocks.Store) {},
			SetupClient:   func(client *clientmocks.Client) {},
			ExpectedError: "invalid channel type, chatID is only available for direct messages and group messages",
		},
		{
			Name: "GetChatIDForChannel: Unable to get the channel members",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(nil, testutils.GetInternalServerAppError("unable to get the channel members")).Times(1)
				api.On("LogError", "Unable to get MM channel members", "channelID", testutils.GetChannelID(), "error", "unable to get the channel members")
			},
			SetupStore:    func(store *storemocks.Store) {},
			SetupClient:   func(client *clientmocks.Client) {},
			ExpectedError: "unable to get the channel members",
		},
		{
			Name: "GetChatIDForChannel: Unable to store users",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", errors.New("unable to store the user")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client) {},
			ExpectedError: "unable to store the user",
		},
		{
			Name: "GetChatIDForChannel: Unable to create or get chat for users",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("LogError", "Unable to create or get chat for users", "error", "unable to create or get chat for users")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("mockTeamsUserID", nil).Times(2)
				store.On("GetTokenForMattermostUser", "mockClientUserID").Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("CreateOrGetChatForUsers", []string{"mockTeamsUserID", "mockTeamsUserID"}).Return(nil, errors.New("unable to create or get chat for users")).Times(1)
			},
			ExpectedError: "unable to create or get chat for users",
		},
		{
			Name: "GetChatIDForChannel: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("mockTeamsUserID", nil).Times(2)
				store.On("GetTokenForMattermostUser", "mockClientUserID").Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("CreateOrGetChatForUsers", []string{"mockTeamsUserID", "mockTeamsUserID"}).Return(mockChat, nil).Times(1)
			},
			ExpectedResult: testutils.GetChatID(),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			client := p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client)
			test.SetupClient(client)
			resp, err := p.GetChatIDForChannel(client, testutils.GetChannelID())
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
				assert.Equal(resp, "")
			} else {
				assert.Nil(err)
				assert.Equal(resp, test.ExpectedResult)
			}
		})
	}
}

func TestGetMentionsData(t *testing.T) {
	for _, test := range []struct {
		Name                  string
		Message               string
		ChatID                string
		SetupAPI              func(*plugintest.API)
		SetupStore            func(*storemocks.Store)
		SetupClient           func(*clientmocks.Client)
		ExpectedMessage       string
		ExpectedMentionsCount int
	}{
		{
			Name:            "GetMentionsData: mentioned in direct chat message",
			Message:         "Hi @all",
			ExpectedMessage: "Hi @all",
			ChatID:          testutils.GetChatID(),
			SetupAPI:        func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{}, nil)
			},
			SetupStore: func(store *storemocks.Store) {},
		},
		{
			Name:            "GetMentionsData: mentioned all in a group chat message",
			Message:         "Hi @all",
			ExpectedMessage: "Hi <at id=\"0\">Everyone</at>",
			ChatID:          testutils.GetChatID(),
			SetupAPI:        func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					Type: "G",
				}, nil)
			},
			SetupStore:            func(store *storemocks.Store) {},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: error occurred while getting chat",
			Message:         "Hi @all",
			ExpectedMessage: "Hi <at id=\"0\">@all</at>",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("LogDebug", "Unable to get ms teams chat", "Error", "error occurred while getting chat")
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(nil, errors.New("error occurred while getting chat"))
			},
			SetupStore:            func(store *storemocks.Store) {},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: mentioned all in Teams channel",
			Message:         "Hi @all",
			ExpectedMessage: "Hi <at id=\"0\">mock-name</at>",
			SetupAPI:        func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChannelInTeam", testutils.GetTeamID(), testutils.GetChannelID()).Return(&clientmodels.Channel{
					DisplayName: "mock-name",
				}, nil)
			},
			SetupStore:            func(store *storemocks.Store) {},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: error occurred while getting the MS Teams channel",
			Message:         "Hi @all",
			ExpectedMessage: "Hi <at id=\"0\">@all</at>",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogDebug", "Unable to get ms teams channel", "Error", "error occurred while getting the MS Teams channel")
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChannelInTeam", testutils.GetTeamID(), testutils.GetChannelID()).Return(nil, errors.New("error occurred while getting the MS Teams channel"))
			},
			SetupStore:            func(store *storemocks.Store) {},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: mentioned a user",
			Message:         "Hi @test-username",
			ExpectedMessage: "Hi <at id=\"0\">mock-name</at>",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByUsername", "test-username").Return(testutils.GetUser("mock-role", "mock-email"), nil)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetUser", testutils.GetUserID()).Return(&clientmodels.User{
					DisplayName: "mock-name",
				}, nil)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetUserID(), nil)
			},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: mentioned all and a specific user in a group chat",
			Message:         "Hi @all @test-username",
			ExpectedMessage: "Hi <at id=\"0\">Everyone</at> <at id=\"1\">mock-name</at>",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByUsername", "test-username").Return(testutils.GetUser("mock-role", "mock-email"), nil)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					Type: "G",
				}, nil)
				client.On("GetUser", testutils.GetUserID()).Return(&clientmodels.User{
					DisplayName: "mock-name",
				}, nil)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetUserID(), nil)
			},
			ExpectedMentionsCount: 2,
		},
		{
			Name:            "GetMentionsData: error getting MM user with username",
			Message:         "Hi @test-username",
			ExpectedMessage: "Hi @test-username",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("LogDebug", "Unable to get user by username", "Error", "error getting MM user with username")
				api.On("GetUserByUsername", "test-username").Return(nil, testutils.GetInternalServerAppError("error getting MM user with username"))
			},
			SetupClient: func(client *clientmocks.Client) {},
			SetupStore:  func(store *storemocks.Store) {},
		},
		{
			Name:            "GetMentionsData: error getting msteams user ID from MM user ID",
			Message:         "Hi @test-username",
			ExpectedMessage: "Hi @test-username",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("LogDebug", "Unable to get msteams user ID", "Error", "error getting msteams user ID from MM user ID")
				api.On("GetUserByUsername", "test-username").Return(testutils.GetUser("mock-role", "mock-email"), nil)
			},
			SetupClient: func(client *clientmocks.Client) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", errors.New("error getting msteams user ID from MM user ID"))
			},
		},
		{
			Name:            "GetMentionsData: error getting msteams user",
			Message:         "Hi @test-username",
			ExpectedMessage: "Hi @test-username",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("LogDebug", "Unable to get msteams user", "Error", "error getting msteams user")
				api.On("GetUserByUsername", "test-username").Return(testutils.GetUser("mock-role", "mock-email"), nil)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetUser", testutils.GetUserID()).Return(nil, errors.New("error getting msteams user"))
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetUserID(), nil)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))

			client := p.msteamsAppClient.(*clientmocks.Client)
			test.SetupClient(client)

			msg, mentions := p.getMentionsData(test.Message, testutils.GetTeamID(), testutils.GetChannelID(), test.ChatID, client)
			assert.Equal(test.ExpectedMessage, msg)
			assert.Equal(test.ExpectedMentionsCount, len(mentions))
		})
	}
}
