package main

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	mockClient "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	mockStore "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExecuteUnlinkCommand(t *testing.T) {
	p := newTestPlugin()
	mockAPI := &plugintest.API{}

	for _, testCase := range []struct {
		description string
		args        *model.CommandArgs
		setupAPI    func(*plugintest.API)
		setupStore  func(*mockStore.Store)
	}{
		{
			description: "Successfully executed unlinked command",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Id:   testutils.GetChannelID(),
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "The MS Teams channel is no longer linked to this Mattermost channel",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("DeleteLinkByChannelID", testutils.GetChannelID()).Return(nil).Times(1)
			},
		},
		{
			description: "Unable to delete link.",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: "Mock-ChannelID",
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", "Mock-ChannelID").Return(&model.Channel{
					Id:   "Mock-ChannelID",
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", testutils.GetUserID(), "Mock-ChannelID", model.PermissionManageChannelRoles).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: "Mock-ChannelID",
					Message:   "Unable to delete link.",
				}).Return(testutils.GetPost("Mock-ChannelID", testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("DeleteLinkByChannelID", "Mock-ChannelID").Return(errors.New("Error while deleting a link")).Times(1)
			},
		},
		{
			description: "Unable to get the current channel",
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", "").Return(nil, testutils.GetInternalServerAppError("Error while getting the current channel.")).Once()
				api.On("SendEphemeralPost", "", &model.Post{
					UserId:  "bot-user-id",
					Message: "Unable to get the current channel information.",
				}).Return(testutils.GetPost("", "")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {},
		},
		{
			description: "Unable to unlink channel as user is not a channel admin.",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Id:   testutils.GetChannelID(),
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(false).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					ChannelId: testutils.GetChannelID(),
					UserId:    "bot-user-id",
					Message:   "Unable to unlink the channel, you have to be a channel admin to unlink it.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {},
		},
		{
			description: "Unable to unlink channel as channel is either a direct or group message",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Id:   testutils.GetChannelID(),
					Type: model.ChannelTypeDirect,
				}, nil).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					ChannelId: testutils.GetChannelID(),
					UserId:    "bot-user-id",
					Message:   "Linking/unlinking a direct or group message is not allowed",
				}).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			testCase.setupAPI(mockAPI)
			p.SetAPI(mockAPI)

			testCase.setupStore(p.store.(*mockStore.Store))
			_, _ = p.executeUnlinkCommand(&plugin.Context{}, testCase.args)
		})
	}
}

func TestExecuteShowCommand(t *testing.T) {
	p := newTestPlugin()
	mockAPI := &plugintest.API{}

	for _, testCase := range []struct {
		description string
		args        *model.CommandArgs
		setupAPI    func(*plugintest.API)
		setupStore  func(*mockStore.Store)
		setupClient func(*mockClient.Client)
	}{
		{
			description: "Successfully executed show command",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "This channel is linked to the MS Teams Channel \"\" (with id: ) in the Team \"\" (with the id: ).",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MSTeamsTeam: "Valid-MSTeamsTeam",
				}, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {
				c.On("GetTeam", "Valid-MSTeamsTeam").Return(&msteams.Team{}, nil).Times(1)
				c.On("GetChannel", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&msteams.Channel{}, nil).Times(1)
			},
		},
		{
			description: "Unable to get the link",
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", "", &model.Post{
					UserId:  "bot-user-id",
					Message: "Link doesn't exists.",
				}).Return(testutils.GetPost("", "")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", "").Return(nil, errors.New("Error while getting the link")).Times(1)
			},
			setupClient: func(c *mockClient.Client) {},
		},
		{
			description: "Unable to get the MS Teams team information",
			args: &model.CommandArgs{
				ChannelId: "Invalid-ChannelID",
			},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", "", &model.Post{
					UserId:    "bot-user-id",
					ChannelId: "Invalid-ChannelID",
					Message:   "Unable to get the MS Teams team information.",
				}).Return(testutils.GetPost("Invalid-ChannelID", "")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", "Invalid-ChannelID").Return(&storemodels.ChannelLink{
					MSTeamsTeam: "Invalid-MSTeamsTeam",
				}, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {
				c.On("GetTeam", "Invalid-MSTeamsTeam").Return(nil, errors.New("Error while getting the MS Teams team information")).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			testCase.setupAPI(mockAPI)
			p.SetAPI(mockAPI)

			testCase.setupStore(p.store.(*mockStore.Store))
			testCase.setupClient(p.msteamsAppClient.(*mockClient.Client))
			_, _ = p.executeShowCommand(&plugin.Context{}, testCase.args)
		})
	}
}

func TestExecuteDisconnectCommand(t *testing.T) {
	p := newTestPlugin()
	mockAPI := &plugintest.API{}

	for _, testCase := range []struct {
		description string
		args        *model.CommandArgs
		setupAPI    func(*plugintest.API)
		setupStore  func(*mockStore.Store)
	}{
		{
			description: "Successfully account disconnected",
			args: &model.CommandArgs{
				UserId: testutils.GetUserID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:  "bot-user-id",
					Message: "Your account has been disconnected",
				}).Return(testutils.GetPost("", testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", testutils.GetUserID()).Return(testutils.GetTeamUserID(), nil).Times(1)
				var token *oauth2.Token
				s.On("SetUserInfo", testutils.GetUserID(), testutils.GetTeamUserID(), token).Return(nil).Times(1)
			},
		},
		{
			description: "User account is not connected",
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", "", &model.Post{
					UserId:  "bot-user-id",
					Message: "Error: the account is not connected",
				}).Return(testutils.GetPost("", "")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "").Return("", errors.New("Unable to get team UserID")).Times(1)
			},
		},
		{
			description: "Unable to disconnect your account",
			args: &model.CommandArgs{
				UserId: testutils.GetUserID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:  "bot-user-id",
					Message: "Error: unable to disconnect your account, Error while disconnecting your account",
				}).Return(testutils.GetPost("", testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", testutils.GetUserID()).Return("", nil).Times(1)
				var token *oauth2.Token
				s.On("SetUserInfo", testutils.GetUserID(), "", token).Return(errors.New("Error while disconnecting your account")).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			testCase.setupAPI(mockAPI)
			p.SetAPI(mockAPI)

			testCase.setupStore(p.store.(*mockStore.Store))
			_, _ = p.executeDisconnectCommand(&plugin.Context{}, testCase.args)
		})
	}
}

func TestExecuteDisconnectBotCommand(t *testing.T) {
	p := newTestPlugin()
	mockAPI := &plugintest.API{}

	for _, testCase := range []struct {
		description string
		args        *model.CommandArgs
		setupAPI    func(*plugintest.API)
		setupStore  func(*mockStore.Store)
	}{
		{
			description: "Successfully bot account disconnected",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "The bot account has been disconnected",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetUserID(), nil).Times(1)
				var token *oauth2.Token
				s.On("SetUserInfo", "bot-user-id", testutils.GetUserID(), token).Return(nil).Times(1)
			},
		},
		{
			description: "Unable to find the connected bot account",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "Error: unable to find the connected bot account",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "bot-user-id").Return("", errors.New("Error: unable to find the connected bot account")).Times(1)
			},
		},
		{
			description: "Unable to disconnect the bot account",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "Error: unable to disconnect the bot account, Error while disconnecting the bot account",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetUserID(), nil).Times(1)
				var token *oauth2.Token
				s.On("SetUserInfo", "bot-user-id", testutils.GetUserID(), token).Return(errors.New("Error while disconnecting the bot account")).Times(1)
			},
		},
		{
			description: "Unable to connect the bot account",
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", "", model.PermissionManageSystem).Return(false).Times(1)
				api.On("SendEphemeralPost", "", &model.Post{
					UserId:  "bot-user-id",
					Message: "Unable to connect the bot account, only system admins can connect the bot account.",
				}).Return(testutils.GetPost("", "")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p.SetAPI(mockAPI)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(p.store.(*mockStore.Store))

			_, _ = p.executeDisconnectBotCommand(&plugin.Context{}, testCase.args)
		})
	}
}

func TestExecuteLinkCommand(t *testing.T) {
	p := newTestPlugin()
	mockAPI := &plugintest.API{}

	for _, testCase := range []struct {
		description string
		parameters  []string
		args        *model.CommandArgs
		setupAPI    func(*plugintest.API)
		setupStore  func(*mockStore.Store)
		setupClient func(*mockClient.Client, *mockClient.Client)
	}{
		{
			description: "Successfully executed link command",
			parameters:  []string{testutils.GetTeamUserID(), testutils.GetChannelID()},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				TeamId:    testutils.GetTeamUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "The MS Teams channel is now linked to this Mattermost channel",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamUserID()).Return(true).Times(1)
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetLinkByMSTeamsChannelID", testutils.GetTeamUserID(), testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
				s.On("StoreChannelLink", mock.Anything).Return(nil).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {
				uc.On("GetChannel", testutils.GetTeamUserID(), testutils.GetChannelID()).Return(&msteams.Channel{}, nil)
			},
		},
		{
			description: "Unable to link a MS Teams channel to multiple channels",
			parameters:  []string{testutils.GetTeamUserID(), testutils.GetChannelID()},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				TeamId:    testutils.GetTeamUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "A link to this MS Teams channel already exists, please remove unlink the channel before you link a new one",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamUserID()).Return(true).Times(1)
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetLinkByMSTeamsChannelID", testutils.GetTeamUserID(), testutils.GetChannelID()).Return(&storemodels.ChannelLink{}, nil).Times(1)
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
				s.On("StoreChannelLink", mock.Anything).Return(nil).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {
				uc.On("GetChannel", testutils.GetTeamUserID(), testutils.GetChannelID()).Return(&msteams.Channel{}, nil)
			},
		},
		{
			description: "Invalid link command",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				TeamId:    testutils.GetTeamUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "Invalid link command, please pass the MS Teams team id and channel id as parameters.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			setupStore:  func(s *mockStore.Store) {},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {},
		},
		{
			description: "Team is not enabled for MS Teams sync",
			parameters:  []string{"", ""},
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", "").Return(&model.Channel{
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", "", "", model.PermissionManageChannelRoles).Return(true).Times(1)
				api.On("SendEphemeralPost", "", &model.Post{
					UserId:  "bot-user-id",
					Message: "This team is not enabled for MS Teams sync.",
				}).Return(testutils.GetPost("", "")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", "").Return(false).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {},
		},
		{
			description: "Unable to get the current channel information",
			parameters:  []string{testutils.GetTeamUserID(), ""},
			args: &model.CommandArgs{
				TeamId: testutils.GetTeamUserID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", "").Return(nil, testutils.GetInternalServerAppError("Error while getting the current channel.")).Times(1)
				api.On("SendEphemeralPost", "", &model.Post{
					UserId:  "bot-user-id",
					Message: "Unable to get the current channel information.",
				}).Return(testutils.GetPost("", "")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamUserID()).Return(true).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {},
		},
		{
			description: "Unable to link the channel as only channel admin can link it",
			parameters:  []string{testutils.GetTeamUserID(), ""},
			args: &model.CommandArgs{
				TeamId:    testutils.GetTeamUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", "", testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(false).Times(1)
				api.On("SendEphemeralPost", "", &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "Unable to link the channel. You have to be a channel admin to link it.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), "")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamUserID()).Return(true).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {},
		},
		{
			description: "Unable to link channel as channel is either a direct or group message",
			parameters:  []string{testutils.GetTeamUserID(), ""},
			args: &model.CommandArgs{
				TeamId:    testutils.GetTeamUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Type: model.ChannelTypeGroup,
				}, nil).Times(1)
				api.On("SendEphemeralPost", "", &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "Linking/unlinking a direct or group message is not allowed",
				}).Return(testutils.GetPost(testutils.GetChannelID(), "")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamUserID()).Return(true).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {},
		},
		{
			description: "Unable to find MS Teams channel as user don't have the permissions to access it",
			parameters:  []string{testutils.GetTeamUserID(), ""},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				TeamId:    testutils.GetTeamUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "MS Teams channel not found or you don't have the permissions to access it.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamUserID()).Return(true).Times(1)
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetLinkByMSTeamsChannelID", testutils.GetTeamUserID(), "").Return(nil, nil).Times(1)
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {
				uc.On("GetChannel", testutils.GetTeamUserID(), "").Return(nil, errors.New("Error while getting the channel"))
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p.SetAPI(mockAPI)
			testCase.setupAPI(mockAPI)

			testCase.setupStore(p.store.(*mockStore.Store))
			testCase.setupClient(p.msteamsAppClient.(*mockClient.Client), p.clientBuilderWithToken("", "", nil, nil).(*mockClient.Client))
			_, _ = p.executeLinkCommand(&plugin.Context{}, testCase.args, testCase.parameters)
		})
	}
}

func TestGetAutocompleteData(t *testing.T) {
	for _, testCase := range []struct {
		description      string
		autocompleteData *model.AutocompleteData
	}{
		{
			description: "Successfully get all auto complete data",
			autocompleteData: &model.AutocompleteData{
				Trigger:   "msteams-sync",
				Hint:      "[command]",
				HelpText:  "Manage MS Teams linked channels",
				RoleID:    model.SystemUserRoleId,
				Arguments: []*model.AutocompleteArg{},
				SubCommands: []*model.AutocompleteData{
					{
						Trigger:  "link",
						Hint:     "[msteams-team-id] [msteams-channel-id]",
						HelpText: "Link current channel to a MS Teams channel",
						RoleID:   model.SystemUserRoleId,
						Arguments: []*model.AutocompleteArg{
							{
								HelpText: "[msteams-team-id]",
								Type:     "DynamicList",
								Required: true,
								Data: &model.AutocompleteDynamicListArg{
									FetchURL: "plugins/com.mattermost.msteams-sync/autocomplete/teams",
								},
							},
							{
								HelpText: "[msteams-channel-id]",
								Type:     "DynamicList",
								Required: true,
								Data: &model.AutocompleteDynamicListArg{
									FetchURL: "plugins/com.mattermost.msteams-sync/autocomplete/channels",
								},
							},
						},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "unlink",
						HelpText:    "Unlink the current channel from the MS Teams channel",
						RoleID:      model.SystemUserRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "show",
						HelpText:    "Show MS Teams linked channel",
						RoleID:      model.SystemUserRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "connect",
						HelpText:    "Connect your Mattermost account to your MS Teams account",
						RoleID:      model.SystemUserRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "disconnect",
						HelpText:    "Disconnect your Mattermost account from your MS Teams account",
						RoleID:      model.SystemUserRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "connect-bot",
						HelpText:    "Connect the bot account (only system admins can do this)",
						RoleID:      model.SystemAdminRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "disconnect-bot",
						HelpText:    "Disconnect the bot account (only system admins can do this)",
						RoleID:      model.SystemAdminRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
				},
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			autocompleteData := getAutocompleteData()
			assert.Equal(t, testCase.autocompleteData, autocompleteData)
		})
	}
}
