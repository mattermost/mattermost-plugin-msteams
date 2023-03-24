package main

import (
	"errors"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"
)

func TestMessageWillBePosted(t *testing.T) {
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		ExpectedMessage string
		ExpectedPost    *model.Post
	}{
		{
			Name: "MessageWillBePosted: Unable to get the channel",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			ExpectedPost: testutils.GetPost(),
		},
		{
			Name: "MessageWillBePosted: Unable to get the channel members",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, 10).Return(nil, testutils.GetInternalServerAppError("unable to get the channel members")).Times(1)
			},
			ExpectedPost: testutils.GetPost(),
		},
		{
			Name: "MessageWillBePosted: Unable to get the user",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, 10).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the user")).Times(1)
			},
			ExpectedPost: testutils.GetPost(),
		},
		{
			Name: "MessageWillBePosted: User email with suffix '@msteamssync'",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, 10).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@msteamssync"), nil).Times(2)
				api.On("SendEphemeralPost", testutils.GetID(), mock.Anything).Return(nil).Times(1)
			},
			ExpectedMessage: "Attachments not supported in direct messages with MSTeams members",
		},
		{
			Name: "MessageWillBePosted: User with different email",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, 10).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(2)
			},
			ExpectedPost: testutils.GetPost(),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin()
			p.configuration.SyncDirectMessages = true
			test.SetupAPI(p.API.(*plugintest.API))
			post, resp := p.MessageWillBePosted(&plugin.Context{}, testutils.GetPost())

			assert.Equal(test.ExpectedMessage, resp)
			assert.Equal(test.ExpectedPost, post)
		})
	}
}

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
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogWarn", "Error creating post", "error", "unable to set the reaction")
				api.On("LogError", "Unable to handle message reaction set", "error", "unable to set the reaction")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{MattermostID: testutils.GetID(), MSTeamsID: "ms-teams-id", MSTeamsChannel: "ms-teams-channel-id", MSTeamsLastUpdateAt: time.UnixMicro(100)}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{MattermostTeam: "mm-team-id", MattermostChannel: "mm-channel-id", MSTeamsTeam: "ms-teams-team-id", MSTeamsChannel: "ms-teams-channel-id"}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SetReaction", "ms-teams-team-id", "ms-teams-channel-id", "", "ms-teams-id", testutils.GetID(), mock.AnythingOfType("string")).Return(errors.New("unable to set the reaction")).Times(1)
			},
		},
		{
			Name: "ReactionHasBeenAdded: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{MattermostID: testutils.GetID(), MSTeamsID: "ms-teams-id", MSTeamsChannel: "ms-teams-channel-id", MSTeamsLastUpdateAt: time.UnixMicro(100)}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{MattermostTeam: "mm-team-id", MattermostChannel: "mm-channel-id", MSTeamsTeam: "ms-teams-team-id", MSTeamsChannel: "ms-teams-channel-id"}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SetReaction", "ms-teams-team-id", "ms-teams-channel-id", "", "ms-teams-id", testutils.GetID(), mock.AnythingOfType("string")).Return(nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin()
			p.configuration.SyncDirectMessages = true
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", nil, nil).(*clientmocks.Client))
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
			Name: "ReactionHasBeenRemoved: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Removing reaction hook", "reaction", mock.Anything).Times(1)
			},
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
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to get the link by channel ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to handle message reaction unset", "error", mock.Anything).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("MattermostToTeamsUserID", mock.AnythingOfType("string")).Return("", testutils.GetInternalServerAppError("unable to get source user ID")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
		},
		{
			Name: "ReactionHasBeenRemoved: Unable to get the link by channel ID and channel",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
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
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", testutils.GetMockArgumentsWithType("string", 6)...).Return(errors.New("unable to set the reaction")).Times(1)
			},
		},
		{
			Name: "ReactionHasBeenRemoved: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetPost", testutils.GetID()).Return(testutils.GetPost(), nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", testutils.GetMockArgumentsWithType("string", 6)...).Return(nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin()
			p.configuration.SyncDirectMessages = true
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", nil, nil).(*clientmocks.Client))
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
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, 10).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
				store.On("MattermostToTeamsUserID", mock.AnythingOfType("string")).Return(testutils.GetID(), nil).Times(2)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(2)
				store.On("LinkPosts", mock.AnythingOfType("storemodels.PostInfo")).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.Anything).Return(testutils.GetID(), nil).Times(1)
				uclient.On("UpdateChatMessage", testutils.GetMockArgumentsWithType("string", 3)...).Return(nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetMockArgumentsWithType("string", 2)...).Return(&msteams.Message{}, nil).Times(1)
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
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, 10).Return(nil, testutils.GetInternalServerAppError("unable to get channel members")).Times(1)
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
				api.On("LogError", mock.AnythingOfType("string"), "error", mock.Anything).Times(2)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, 10).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
				store.On("MattermostToTeamsUserID", mock.AnythingOfType("string")).Return(testutils.GetID(), nil).Times(2)
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
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, 10).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
				store.On("MattermostToTeamsUserID", mock.AnythingOfType("string")).Return(testutils.GetID(), nil).Times(2)
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
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}).Return(nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", testutils.GetMockArgumentsWithType("string", 5)...).Return(nil).Times(1)
				uclient.On("GetMessage", testutils.GetMockArgumentsWithType("string", 3)...).Return(&msteams.Message{}, nil).Times(1)
			},
		},
		{
			Name: "MessageHasBeenUpdated: Able to get the link by channel ID but unable to update post",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogWarn", "Error updating the post", "error", mock.Anything).Return(nil).Times(1)
				api.On("LogError", "Unable to handle message update", "error", mock.Anything).Return(nil).Times(1)
				api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}).Return(nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", testutils.GetMockArgumentsWithType("string", 5)...).Return(errors.New("unable to update the post")).Times(1)
				uclient.On("GetMessage", testutils.GetMockArgumentsWithType("string", 3)...).Return(&msteams.Message{}, nil).Times(1)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin()
			p.configuration.SyncDirectMessages = true
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", nil, nil).(*clientmocks.Client))
			p.MessageHasBeenUpdated(&plugin.Context{}, testutils.GetPost(), testutils.GetPost())
		})
	}
}
