package main

import (
	"errors"
	"testing"
	"time"

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
			p := newTestPlugin()
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", nil, nil).(*clientmocks.Client))
			err := p.Delete("mockTeamsTeamID", testutils.GetChannelID(), testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), testutils.GetPost())
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
			p := newTestPlugin()
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", nil, nil).(*clientmocks.Client))
			err := p.DeleteChat("mockChatID", testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), testutils.GetPost())
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}
