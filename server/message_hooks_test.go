package main

import (
	"bytes"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"
)

func TestReactionHasBeenAdded(t *testing.T) {
	for _, test := range []struct {
		Name        string
		SetupAPI    func(*plugintest.API)
		SetupStore  func(*storemocks.Store)
		SetupClient func(*clientmocks.Client, *clientmocks.Client)
	}{
		{
			Name:     "ReactionHasBeenAdded: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client, uc *clientmocks.Client) {},
		},
		{
			Name: "ReactionHasBeenAdded: Unable to get the link by channel ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("LogError", "Unable to handle message reaction set", "error", mock.Anything).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("MattermostToTeamsUserID", mock.AnythingOfType("string")).Return("", testutils.GetInternalServerAppError("unable to get the source user ID")).Times(1)
			},
			SetupClient: func(c *clientmocks.Client, uc *clientmocks.Client) {},
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
		},
		{
			Name: "ReactionHasBeenAdded: Unable to set the reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogWarn", "Error setting reaction", "error", "unable to set the reaction")
				api.On("LogError", "Unable to handle message reaction set", "error", "unable to set the reaction")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{MattermostID: testutils.GetID(), MSTeamsID: "ms-teams-id", MSTeamsChannel: "ms-teams-channel-id", MSTeamsLastUpdateAt: time.UnixMicro(100)}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{MattermostTeamID: "mm-team-id", MattermostChannelID: "mm-channel-id", MSTeamsTeam: "ms-teams-team-id", MSTeamsChannel: "ms-teams-channel-id"}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SetReaction", "ms-teams-team-id", "ms-teams-channel-id", "", "ms-teams-id", testutils.GetID(), mock.AnythingOfType("string")).Return(errors.New("unable to set the reaction")).Times(1)
			},
		},
		{
			Name: "ReactionHasBeenAdded: Unable to get the post metadata",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogWarn", "Error getting the msteams post metadata", "error", "unable to get post info")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{MattermostID: testutils.GetID(), MSTeamsID: "ms-teams-id", MSTeamsChannel: "ms-teams-channel-id", MSTeamsLastUpdateAt: time.UnixMicro(100)}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{MattermostTeamID: "mm-team-id", MattermostChannelID: "mm-channel-id", MSTeamsTeam: "ms-teams-team-id", MSTeamsChannel: "ms-teams-channel-id"}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SetReaction", "ms-teams-team-id", "ms-teams-channel-id", "", "ms-teams-id", testutils.GetID(), mock.AnythingOfType("string")).Return(nil).Times(1)
				uclient.On("GetMessage", "ms-teams-team-id", "ms-teams-channel-id", "ms-teams-id").Return(nil, errors.New("unable to get post info")).Times(1)
			},
		},
		{
			Name: "ReactionHasBeenAdded: Unable to set the post last updateAt time",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", "unable to set post lastUpdateAt value")
			},
			SetupStore: func(store *storemocks.Store) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{MattermostID: testutils.GetID(), MSTeamsID: "ms-teams-id", MSTeamsChannel: "ms-teams-channel-id", MSTeamsLastUpdateAt: time.UnixMicro(100)}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{MattermostTeamID: "mm-team-id", MattermostChannelID: "mm-channel-id", MSTeamsTeam: "ms-teams-team-id", MSTeamsChannel: "ms-teams-channel-id"}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", testutils.GetID(), demoTime).Return(errors.New("unable to set post lastUpdateAt value")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				uclient.On("SetReaction", "ms-teams-team-id", "ms-teams-channel-id", "", "ms-teams-id", testutils.GetID(), mock.AnythingOfType("string")).Return(nil).Times(1)
				uclient.On("GetMessage", "ms-teams-team-id", "ms-teams-channel-id", "ms-teams-id").Return(&msteams.Message{LastUpdateAt: demoTime}, nil).Times(1)
			},
		},
		{
			Name: "ReactionHasBeenAdded: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{MattermostID: testutils.GetID(), MSTeamsID: "ms-teams-id", MSTeamsChannel: "ms-teams-channel-id", MSTeamsLastUpdateAt: time.UnixMicro(100)}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{MattermostTeamID: "mm-team-id", MattermostChannelID: "mm-channel-id", MSTeamsTeam: "ms-teams-team-id", MSTeamsChannel: "ms-teams-channel-id"}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", testutils.GetID(), demoTime).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				uclient.On("SetReaction", "ms-teams-team-id", "ms-teams-channel-id", "", "ms-teams-id", testutils.GetID(), mock.AnythingOfType("string")).Return(nil).Times(1)
				uclient.On("GetMessage", "ms-teams-team-id", "ms-teams-channel-id", "ms-teams-id").Return(&msteams.Message{LastUpdateAt: demoTime}, nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			p.configuration.SyncDirectMessages = true
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			p.ReactionHasBeenAdded(&plugin.Context{}, testutils.GetReaction())
		})
	}
}

func TestReactionHasBeenRemoved(t *testing.T) {
	for _, test := range []struct {
		Name        string
		SetupAPI    func(*plugintest.API)
		SetupStore  func(*storemocks.Store)
		SetupClient func(*clientmocks.Client, *clientmocks.Client)
	}{
		{
			Name:     "ReactionHasBeenRemoved: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to get the post",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get the post from the reaction", "reaction", mock.Anything, "error", mock.Anything).Times(1)
				api.On("GetPost", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to get the link by channel ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to handle message reaction unset", "error", mock.Anything).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(1)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", testutils.GetInternalServerAppError("unable to get source user ID")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to get the link by channel ID and channel",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(1)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to remove the reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error creating post", "error", mock.Anything).Times(1)
				api.On("LogError", "Unable to handle message reaction unset", "error", mock.Anything).Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
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
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), mock.AnythingOfType("string")).Return(errors.New("unable to set the reaction")).Times(1)
			},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to get the post metadata",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
				api.On("LogWarn", "Error getting the msteams post metadata", "error", "unable to get post info")
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
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), mock.AnythingOfType("string")).Return(nil).Times(1)
				uclient.On("GetMessage", "mockTeamsTeamID", "mockTeamsChannelID", "").Return(nil, errors.New("unable to get post info")).Times(1)
			},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to set the post last updateAt time",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", "unable to set post lastUpdateAt value")
			},
			SetupStore: func(store *storemocks.Store) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MattermostTeamID:    "mockMattermostTeam",
					MattermostChannelID: "mockMattermostChannel",
					MSTeamsTeam:         "mockTeamsTeamID",
					MSTeamsChannel:      "mockTeamsChannelID",
				}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", testutils.GetID(), demoTime).Return(errors.New("unable to set post lastUpdateAt value")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), mock.AnythingOfType("string")).Return(nil).Times(1)
				uclient.On("GetMessage", "mockTeamsTeamID", "mockTeamsChannelID", "").Return(&msteams.Message{LastUpdateAt: demoTime}, nil).Times(1)
			},
		},
		{
			Name: "ReactionHasBeenRemoved: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			SetupStore: func(store *storemocks.Store) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MattermostTeamID:    "mockMattermostTeam",
					MattermostChannelID: "mockMattermostChannel",
					MSTeamsTeam:         "mockTeamsTeamID",
					MSTeamsChannel:      "mockTeamsChannelID",
				}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", testutils.GetID(), demoTime).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), mock.AnythingOfType("string")).Return(nil).Times(1)
				uclient.On("GetMessage", "mockTeamsTeamID", "mockTeamsChannelID", "").Return(&msteams.Message{LastUpdateAt: demoTime}, nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			p.configuration.SyncDirectMessages = true
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			p.ReactionHasBeenRemoved(&plugin.Context{}, testutils.GetReaction())
		})
	}
}

func TestMessageHasBeenUpdated(t *testing.T) {
	for _, test := range []struct {
		Name        string
		SetupAPI    func(*plugintest.API)
		SetupStore  func(*storemocks.Store)
		SetupClient func(*clientmocks.Client, *clientmocks.Client)
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
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("mockChatID", nil).Times(2)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMsgID",
				}, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMsgID",
					MSTeamsChannel: "mockChatID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("UpdateChatMessage", "mockChatID", "mockMsgID", "", []models.ChatMessageMentionable{}).Return(nil).Times(1)
				uclient.On("GetChatMessage", "mockChatID", "mockMsgID").Return(&msteams.Message{}, nil).Times(1)
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
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and channel type is Open",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeOpen), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
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
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and unable to update the chat",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error updating post", "error", mock.Anything).Times(1)
				api.On("LogError", "Unable to handle message update", "error", mock.Anything).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(2)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get post info")).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return(testutils.GetID(), nil).Times(1)
			},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and unable to create or get chat for users",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(2)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("", errors.New("unable to create or get chat for users")).Times(1)
			},
		},
		{
			Name: "MessageHasBeenUpdated: Able to get the link by channel ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
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
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", "mockTeamsChannelID", "", "mockMessageID", "", []models.ChatMessageMentionable{}).Return(nil).Times(1)
				uclient.On("GetMessage", "mockTeamsTeamID", "mockTeamsChannelID", "mockMessageID").Return(&msteams.Message{}, nil).Times(1)
			},
		},
		{
			Name: "MessageHasBeenUpdated: Able to get the link by channel ID but unable to update post",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the post", "error", mock.Anything).Return(nil).Times(1)
				api.On("LogError", "Unable to handle message update", "error", mock.Anything).Return(nil).Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
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
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", "mockTeamsChannelID", "", "", "", []models.ChatMessageMentionable{}).Return(errors.New("unable to update the post")).Times(1)
				uclient.On("GetMessage", "mockTeamsTeamID", "mockTeamsChannelID", "mockMessageID").Return(&msteams.Message{}, nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			p.configuration.SyncDirectMessages = true
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			p.MessageHasBeenUpdated(&plugin.Context{}, testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()))
		})
	}
}

func TestSetChatReaction(t *testing.T) {
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		ExpectedMessage string
	}{
		{
			Name:     "SetChatReaction: Unable to get the source user ID",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", testutils.GetInternalServerAppError("unable to get the source user ID")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
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
			ExpectedMessage: "not connected user",
		},
		{
			Name: "SetChatReaction: Unable to get the chat ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedMessage: "unable to get the channel",
		},
		{
			Name: "SetChatReaction: Unable to set the chat reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error creating post reaction", "error", mock.Anything)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("SetChatReaction", "mockChatID", "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(testutils.GetInternalServerAppError("unable to set the chat reaction")).Times(1)
			},
			ExpectedMessage: "unable to set the chat reaction",
		},
		{
			Name: "SetChatReaction: Unable to get the post metadata",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("LogWarn", "Error getting the msteams post metadata", "error", "unable to get post info")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("SetChatReaction", "mockChatID", "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(nil).Times(1)
				uclient.On("GetChatMessage", "mockChatID", "mockTeamsMessageID").Return(nil, errors.New("unable to get post info")).Once()
			},
		},
		{
			Name: "SetChatReaction: Unable to set the post last updateAt time",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", "unable to set post lastUpdateAt value")
			},
			SetupStore: func(store *storemocks.Store) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", demoTime).Return(errors.New("unable to set post lastUpdateAt value")).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("SetChatReaction", "mockChatID", "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(nil).Times(1)
				uclient.On("GetChatMessage", "mockChatID", "mockTeamsMessageID").Return(&msteams.Message{LastUpdateAt: demoTime}, nil).Once()
			},
		},
		{
			Name: "SetChatReaction: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", demoTime).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("SetChatReaction", "mockChatID", "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(nil).Times(1)
				uclient.On("GetChatMessage", "mockChatID", "mockTeamsMessageID").Return(&msteams.Message{LastUpdateAt: demoTime}, nil).Once()
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			resp := p.SetChatReaction("mockTeamsMessageID", testutils.GetID(), testutils.GetChannelID(), "mockEmojiName")
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}

func TestSetReaction(t *testing.T) {
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		ExpectedMessage string
	}{
		{
			Name:     "SetReaction: Unable to get the post info",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedMessage: "unable to get the post info",
		},
		{
			Name:     "SetReaction: Post info is nil",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
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
			ExpectedMessage: "not connected user",
		},
		{
			Name: "SetReaction: Unable to set the reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error setting reaction", "error", "unable to set the reaction")
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(errors.New("unable to set the reaction")).Times(1)
			},
			ExpectedMessage: "unable to set the reaction",
		},
		{
			Name: "SetReaction: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", "", demoTime).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				uclient.On("SetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(nil).Times(1)
				uclient.On("GetMessage", "mockTeamsTeamID", "mockTeamsChannelID", "").Return(&msteams.Message{LastUpdateAt: demoTime}, nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			p.API.(*plugintest.API).On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			resp := p.SetReaction("mockTeamsTeamID", "mockTeamsChannelID", testutils.GetUserID(), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), "mockName")
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}

func TestUnsetChatReaction(t *testing.T) {
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		ExpectedMessage string
	}{
		{
			Name:     "UnsetChatReaction: Unable to get the source user ID",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", testutils.GetInternalServerAppError("unable to get the source user ID")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
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
			ExpectedMessage: "not connected user",
		},
		{
			Name: "UnsetChatReaction: Unable to get the chat ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "FAILING TO CREATE OR GET THE CHAT", "error", mock.Anything)
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedMessage: "unable to get the channel",
		},
		{
			Name: "UnsetChatReaction: Unable to unset the chat reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error creating post", "error", mock.Anything)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("UnsetChatReaction", "mockChatID", "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(testutils.GetInternalServerAppError("unable to unset the chat reaction")).Times(1)
			},
			ExpectedMessage: "unable to unset the chat reaction",
		},
		{
			Name: "UnsetChatReaction: Unable to get the post metadata",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("LogWarn", "Error getting the msteams post metadata", "error", "unable to get post info")
			},
			SetupStore: func(store *storemocks.Store) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", demoTime).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("UnsetChatReaction", "mockChatID", "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(nil).Times(1)
				uclient.On("GetChatMessage", "mockChatID", "mockTeamsMessageID").Return(nil, errors.New("unable to get post info")).Once()
			},
		},
		{
			Name: "UnsetChatReaction: Unable to set the post last updateAt time",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", "unable to set post lastUpdateAt value")
			},
			SetupStore: func(store *storemocks.Store) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", demoTime).Return(errors.New("unable to set post lastUpdateAt value")).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("UnsetChatReaction", "mockChatID", "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(nil).Times(1)
				uclient.On("GetChatMessage", "mockChatID", "mockTeamsMessageID").Return(&msteams.Message{LastUpdateAt: demoTime}, nil).Once()
			},
		},
		{
			Name: "UnsetChatReaction: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", demoTime).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("UnsetChatReaction", "mockChatID", "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(nil).Times(1)
				uclient.On("GetChatMessage", "mockChatID", "mockTeamsMessageID").Return(&msteams.Message{LastUpdateAt: demoTime}, nil).Once()
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
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
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		ExpectedMessage string
	}{
		{
			Name:     "UnsetReaction: Unable to get the post info",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedMessage: "unable to get the post info",
		},
		{
			Name:     "UnsetReaction: Post info is nil",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
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
			ExpectedMessage: "not connected user",
		},
		{
			Name: "UnsetReaction: Unable to unset the reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error creating post", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(testutils.GetInternalServerAppError("unable to unset the reaction")).Times(1)
			},
			ExpectedMessage: "unable to unset the reaction",
		},
		{
			Name:     "UnsetReaction: Valid",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", "", demoTime).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				demoTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(nil).Times(1)
				uclient.On("GetMessage", "mockTeamsTeamID", "mockTeamsChannelID", "").Return(&msteams.Message{LastUpdateAt: demoTime}, nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			p.API.(*plugintest.API).On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			resp := p.UnsetReaction("mockTeamsTeamID", "mockTeamsChannelID", testutils.GetUserID(), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), "mockName")
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}

func TestSendChat(t *testing.T) {
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		ExpectedMessage string
		ExpectedError   string
	}{
		{
			Name:     "SendChat: Unable to get the source user ID",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", testutils.GetInternalServerAppError("unable to get the source user ID")).Times(1)
				store.On("GetDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID()).Return(time.Now(), errors.New("error in getting prompt from store")).Once()
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "unable to get the source user ID",
		},
		{
			Name: "SendChat: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "Your Mattermost account is not connected to MS Teams so this message will not be relayed to users on MS Teams. You can connect your account using the `/msteams-sync connect` slash command.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID()).Return(time.Now().Add(-24*31*time.Hour), nil).Once()
				store.On("StoreDMAndGMChannelPromptTime", testutils.GetChannelID(), testutils.GetID(), mock.AnythingOfType("time.Time")).Return(errors.New("error in storing prompt")).Once()
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "SendChat: Unable to create or get the chat",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "FAILING TO CREATE OR GET THE CHAT", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("", errors.New("unable to create or get the chat")).Times(1)
			},
			ExpectedError: "unable to create or get the chat",
		},
		{
			Name: "SendChat: Unable to send the chat",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error creating post", "error", mock.Anything)
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFileName"+"_"+testutils.GetID(), 1, "mockMimeType", bytes.NewReader([]byte("mockData"))).Return(&msteams.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendChat", "mockChatID", "", "<p>mockMessage</p>\n", []*msteams.Attachment{{
					ID: testutils.GetID(),
				}}, []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to send the chat")).Times(1)
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
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockMessageID",
				}).Return(testutils.GetInternalServerAppError("unable to store the post")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("SendChat", "mockChatID", "", "<p>mockMessage</p>\n", []*msteams.Attachment{{
					ID: testutils.GetID(),
				}}, []models.ChatMessageMentionable{}).Return(&msteams.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFileName"+"_"+testutils.GetID(), 1, "mockMimeType", bytes.NewReader([]byte("mockData"))).Return(&msteams.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
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
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("SendChat", "mockChatID", "", "<p>mockMessage</p>\n", ([]*msteams.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&msteams.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
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
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("SendChat", "mockChatID", "", "<p>mockMessage</p>\n", ([]*msteams.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&msteams.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "SendChat: Unable to upload the attachments",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error in uploading file attachment to Teams", "error", mock.Anything)
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("SendChat", "mockChatID", "", "<p>mockMessage</p>\n", ([]*msteams.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&msteams.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFileName"+"_"+testutils.GetID(), 1, "mockMimeType", bytes.NewReader([]byte("mockData"))).Return(nil, errors.New("error in uploading attachment")).Times(1)
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
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return("mockChatID", nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFileName"+"_"+testutils.GetID(), 1, "mockMimeType", bytes.NewReader([]byte("mockData"))).Return(&msteams.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendChat", "mockChatID", "", "<p>mockMessage</p>\n", []*msteams.Attachment{{
					ID: testutils.GetID(),
				}}, []models.ChatMessageMentionable{}).Return(&msteams.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
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
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())
			mockPost.Message = "mockMessage"
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
			ExpectedError: "not connected user",
		},
		{
			Name: "Send: Unable to get the file info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Unable to get file info", "error", mock.Anything).Times(1)
				api.On("GetFileInfo", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get file attachment")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage</p>\n", ([]*msteams.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&msteams.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
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
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage</p>\n", ([]*msteams.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&msteams.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "Send: Unable to send message with attachments",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error creating post", "error", mock.Anything).Times(1)
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UploadFile", testutils.GetID(), testutils.GetChannelID(), "mockFileName"+"_"+testutils.GetID(), 1, "mockMimeType", bytes.NewReader([]byte("mockData"))).Return(&msteams.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage</p>\n", []*msteams.Attachment{
					{
						ID: testutils.GetID(),
					},
				}, []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to send message with attachments")).Times(1)
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
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(errors.New("unable to store posts")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UploadFile", testutils.GetID(), testutils.GetChannelID(), "mockFileName"+"_"+testutils.GetID(), 1, "mockMimeType", bytes.NewReader([]byte("mockData"))).Return(&msteams.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage</p>\n", []*msteams.Attachment{
					{
						ID: testutils.GetID(),
					},
				}, []models.ChatMessageMentionable{}).Return(&msteams.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
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
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UploadFile", testutils.GetID(), testutils.GetChannelID(), "mockFileName"+"_"+testutils.GetID(), 1, "mockMimeType", bytes.NewReader([]byte("mockData"))).Return(&msteams.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage</p>\n", []*msteams.Attachment{
					{
						ID: testutils.GetID(),
					},
				}, []models.ChatMessageMentionable{}).Return(&msteams.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
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
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())
			mockPost.Message = "mockMessage"
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
			ExpectedError: "not connected user",
		},
		{
			Name: "Delete: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error updating post", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "Delete: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error deleting post, post not found.")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "post not found",
		},
		{
			Name: "Delete: Unable to delete the message",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error deleting post", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("DeleteMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID").Return(errors.New("unable to delete the message")).Times(1)
			},
			ExpectedError: "unable to delete the message",
		},
		{
			Name:     "Delete: Valid",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("DeleteMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID").Return(nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			err := p.Delete("mockTeamsTeamID", testutils.GetChannelID(), testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()))
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
		ExpectedError string
	}{
		{
			Name:     "DeleteChat: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "DeleteChat: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error updating post", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "DeleteChat: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error deleting post, post not found.")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "post not found",
		},
		{
			Name: "DeleteChat: Unable to delete the message",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error deleting post", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("DeleteChatMessage", "mockChatID", "mockMSTeamsID").Return(errors.New("unable to delete the message")).Times(1)
			},
			ExpectedError: "unable to delete the message",
		},
		{
			Name:     "DeleteChat: Valid",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("DeleteChatMessage", "mockChatID", "mockMSTeamsID").Return(nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			err := p.DeleteChat("mockChatID", testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()))
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	for _, test := range []struct {
		Name          string
		SetupAPI      func(*plugintest.API)
		SetupStore    func(*storemocks.Store)
		SetupClient   func(*clientmocks.Client, *clientmocks.Client)
		ExpectedError string
	}{
		{
			Name:     "Update: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "Update: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error updating post", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "Update: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error updating post, post not found.")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "post not found",
		},
		{
			Name: "Update: Unable to update the message",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the post", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage</p>\n", []models.ChatMessageMentionable{}).Return(errors.New("unable to update the message")).Times(1)
			},
			ExpectedError: "unable to update the message",
		},
		{
			Name: "Update: Unable to get the updated message",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage</p>\n", []models.ChatMessageMentionable{}).Return(nil).Times(1)
				uclient.On("GetMessage", "mockTeamsTeamID", testutils.GetChannelID(), "mockMSTeamsID").Return(nil, errors.New("unable to get the updated message")).Times(1)
			},
		},
		{
			Name: "Update: Unable to store the link posts",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
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
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage</p>\n", []models.ChatMessageMentionable{}).Return(nil).Times(1)
				uclient.On("GetMessage", "mockTeamsTeamID", testutils.GetChannelID(), "mockMSTeamsID").Return(&msteams.Message{
					ID: testutils.GetID(),
				}, nil).Times(1)
			},
		},
		{
			Name: "Update: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
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
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage</p>\n", []models.ChatMessageMentionable{}).Return(nil).Times(1)
				uclient.On("GetMessage", "mockTeamsTeamID", testutils.GetChannelID(), "mockMSTeamsID").Return(&msteams.Message{
					ID: testutils.GetID(),
				}, nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())
			mockPost.Message = "mockMessage"
			err := p.Update("mockTeamsTeamID", testutils.GetChannelID(), testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), mockPost, testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()))
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestUpdateChat(t *testing.T) {
	for _, test := range []struct {
		Name          string
		SetupAPI      func(*plugintest.API)
		SetupStore    func(*storemocks.Store)
		SetupClient   func(*clientmocks.Client, *clientmocks.Client)
		ExpectedError string
	}{
		{
			Name: "UpdateChat: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error updating post", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
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
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "UpdateChat: Unable to update the message",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the post", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage</p>\n", []models.ChatMessageMentionable{}).Return(errors.New("unable to update the message")).Times(1)
			},
			ExpectedError: "unable to update the message",
		},
		{
			Name: "UpdateChat: Unable to get the updated message",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage</p>\n", []models.ChatMessageMentionable{}).Return(nil).Times(1)
				uclient.On("GetChatMessage", "mockChatID", "mockTeamsTeamID").Return(nil, errors.New("unable to get the updated message")).Times(1)
			},
		},
		{
			Name: "UpdateChat: Unable to store the link posts",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
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
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage</p>\n", []models.ChatMessageMentionable{}).Return(nil).Times(1)
				uclient.On("GetChatMessage", "mockChatID", "mockTeamsTeamID").Return(&msteams.Message{
					ID: testutils.GetID(),
				}, nil).Times(1)
			},
		},
		{
			Name: "UpdateChat: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the msteams/mattermost post link metadata", "error", mock.Anything)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
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
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage</p>\n", []models.ChatMessageMentionable{}).Return(nil).Times(1)
				uclient.On("GetChatMessage", "mockChatID", "mockTeamsTeamID").Return(&msteams.Message{
					ID: testutils.GetID(),
				}, nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())
			mockPost.Message = "mockMessage"
			err := p.UpdateChat("mockChatID", testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), mockPost, testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()))
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestGetChatIDForChannel(t *testing.T) {
	for _, test := range []struct {
		Name           string
		SetupAPI       func(*plugintest.API)
		SetupStore     func(*storemocks.Store)
		SetupClient    func(*clientmocks.Client, *clientmocks.Client)
		ExpectedError  string
		ExpectedResult string
	}{
		{
			Name: "GetChatIDForChannel: Unable to get the channel",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore:    func(store *storemocks.Store) {},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "unable to get the channel",
		},
		{
			Name: "GetChatIDForChannel: Channel type is 'open'",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeOpen), nil).Times(1)
			},
			SetupStore:    func(store *storemocks.Store) {},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "invalid channel type, chatID is only available for direct messages and group messages",
		},
		{
			Name: "GetChatIDForChannel: Unable to get the channel members",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(nil, testutils.GetInternalServerAppError("unable to get the channel members")).Times(1)
			},
			SetupStore:    func(store *storemocks.Store) {},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
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
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "unable to store the user",
		},
		{
			Name: "GetChatIDForChannel: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("mockTeamsUserID", nil).Times(2)
				store.On("GetTokenForMattermostUser", "mockClientUserID").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "GetChatIDForChannel: Unable to create or get chat for users",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("mockTeamsUserID", nil).Times(2)
				store.On("GetTokenForMattermostUser", "mockClientUserID").Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", []string{"mockTeamsUserID", "mockTeamsUserID"}).Return("", errors.New("unable to create or get chat for users")).Times(1)
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
				store.On("GetTokenForMattermostUser", "mockClientUserID").Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", []string{"mockTeamsUserID", "mockTeamsUserID"}).Return("mockChatID", nil).Times(1)
			},
			ExpectedResult: "mockChatID",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			resp, err := p.GetChatIDForChannel("mockClientUserID", testutils.GetChannelID())
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
