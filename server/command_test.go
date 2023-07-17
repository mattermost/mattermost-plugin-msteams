package main

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	mockClient "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	mockStore "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExecuteUnlinkCommand(t *testing.T) {
	p := newTestPlugin(t)
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
					Message:   "The MS Teams channel is no longer linked to this Mattermost channel.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Once()
				s.On("DeleteLinkByChannelID", testutils.GetChannelID()).Return(nil).Times(1)
			},
		},
		{
			description: "Unable to get link.",
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
					Message:   "This Mattermost channel is not linked to any MS Teams channel.",
				}).Return(testutils.GetPost("Mock-ChannelID", testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", "Mock-ChannelID").Return(nil, errors.New("Error while getting link")).Once()
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
				s.On("GetLinkByChannelID", "Mock-ChannelID").Return(nil, nil).Once()
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
			_, _ = p.executeUnlinkCommand(testCase.args)
		})
	}
}

func TestExecuteShowCommand(t *testing.T) {
	p := newTestPlugin(t)
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
					MSTeamsTeamID: "Valid-MSTeamsTeamID",
				}, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {
				c.On("GetTeam", "Valid-MSTeamsTeamID").Return(&msteams.Team{}, nil).Times(1)
				c.On("GetChannelInTeam", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&msteams.Channel{}, nil).Times(1)
			},
		},
		{
			description: "Unable to get the link",
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", "", &model.Post{
					UserId:  "bot-user-id",
					Message: "Link doesn't exist.",
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
					MSTeamsTeamID: "Invalid-MSTeamsTeamID",
				}, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {
				c.On("GetTeam", "Invalid-MSTeamsTeamID").Return(nil, errors.New("Error while getting the MS Teams team information")).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			testCase.setupAPI(mockAPI)
			p.SetAPI(mockAPI)

			testCase.setupStore(p.store.(*mockStore.Store))
			testCase.setupClient(p.msteamsAppClient.(*mockClient.Client))
			_, _ = p.executeShowCommand(testCase.args)
		})
	}
}

func TestExecuteShowLinksCommand(t *testing.T) {
	p := newTestPlugin(t)
	mockAPI := &plugintest.API{}

	for _, testCase := range []struct {
		description string
		args        *model.CommandArgs
		setupAPI    func(*plugintest.API)
		setupStore  func(*mockStore.Store)
		setupClient func(*mockClient.Client)
	}{
		{
			description: "Successfully executed show-links command",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()

				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   commandWaitingMessage,
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()

				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "| Mattermost Team | Mattermost Channel | MS Teams Team | MS Teams Channel | \n| :------|:--------|:-------|:-----------|\n|Test MM team|Test MM channel|Test MS team|Test MS channel|",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("ListChannelLinksWithNames").Return(testutils.GetChannelLinks(1), nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {
				c.On("GetTeams", mock.AnythingOfType("string")).Return([]*msteams.Team{{ID: testutils.GetTeamsTeamID(), DisplayName: "Test MS team"}}, nil).Times(1)
				c.On("GetChannelsInTeam", testutils.GetTeamsTeamID(), mock.AnythingOfType("string")).Return([]*msteams.Channel{{ID: testutils.GetTeamsChannelID(), DisplayName: "Test MS channel"}}, nil).Times(1)
			},
		},
		{
			description: "User is not a system admin",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(false).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "Unable to execute the command, only system admins have access to execute this command.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore:  func(s *mockStore.Store) {},
			setupClient: func(c *mockClient.Client) {},
		},
		{
			description: "Error in getting links",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()
				api.On("LogDebug", "Unable to get links from store", "Error", "error in getting links").Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "Something went wrong.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("ListChannelLinksWithNames").Return(nil, errors.New("error in getting links")).Times(1)
			},
			setupClient: func(c *mockClient.Client) {},
		},
		{
			description: "No links present",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "No links present.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("ListChannelLinksWithNames").Return(nil, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {},
		},
		{
			description: "Error in fetching info from MS Teams",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()

				api.On("LogDebug", "Unable to get the MS Teams teams information", "Error", "error in getting teams info").Once()
				api.On("LogDebug", "Unable to get the MS Teams channel information for the team", "TeamID", testutils.GetTeamsTeamID(), "Error", "error in getting channels info").Once()

				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   commandWaitingMessage,
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()

				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "| Mattermost Team | Mattermost Channel | MS Teams Team | MS Teams Channel | \n| :------|:--------|:-------|:-----------|\n|Test MM team|Test MM channel|||\n|Test MM team|Test MM channel|||\n|Test MM team|Test MM channel|||\n|Test MM team|Test MM channel|||\nThere were some errors while fetching information. Please check the server logs.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("ListChannelLinksWithNames").Return(testutils.GetChannelLinks(4), nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {
				c.On("GetTeams", mock.AnythingOfType("string")).Return(nil, errors.New("error in getting teams info")).Times(4)
				c.On("GetChannelsInTeam", testutils.GetTeamsTeamID(), mock.AnythingOfType("string")).Return(nil, errors.New("error in getting channels info")).Times(4)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			testCase.setupAPI(mockAPI)
			p.SetAPI(mockAPI)
			defer mockAPI.AssertExpectations(t)

			testCase.setupStore(p.store.(*mockStore.Store))
			testCase.setupClient(p.msteamsAppClient.(*mockClient.Client))
			_, _ = p.executeShowLinksCommand(testCase.args)
			time.Sleep(1 * time.Second)
		})
	}
}

func TestExecuteDisconnectCommand(t *testing.T) {
	p := newTestPlugin(t)
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
					Message: "Your account has been disconnected.",
				}).Return(testutils.GetPost("", testutils.GetUserID())).Times(1)

				api.On("LogDebug", "Unable to delete the last prompt timestamp for the user", "UserID", testutils.GetUserID(), "Error", "error in deleting prompt time")
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", testutils.GetUserID()).Return(testutils.GetTeamUserID(), nil).Times(1)
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, nil).Once()
				var token *oauth2.Token
				s.On("SetUserInfo", testutils.GetUserID(), testutils.GetTeamUserID(), token).Return(nil).Times(1)
				s.On("DeleteDMAndGMChannelPromptTime", testutils.GetUserID()).Return(errors.New("error in deleting prompt time")).Once()
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
			description: "User account is not connected as token is not found",
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", "", &model.Post{
					UserId:  "bot-user-id",
					Message: "Error: the account is not connected",
				}).Return(testutils.GetPost("", "")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "").Return("", nil).Times(1)
				s.On("GetTokenForMattermostUser", "").Return(nil, errors.New("Unable to get token")).Once()
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
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, nil).Once()
				s.On("SetUserInfo", testutils.GetUserID(), "", token).Return(errors.New("Error while disconnecting your account")).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			testCase.setupAPI(mockAPI)
			p.SetAPI(mockAPI)

			testCase.setupStore(p.store.(*mockStore.Store))
			_, _ = p.executeDisconnectCommand(testCase.args)
		})
	}
}

func TestExecuteDisconnectBotCommand(t *testing.T) {
	p := newTestPlugin(t)
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
					Message:   "The bot account has been disconnected.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetUserID(), nil).Times(1)
				s.On("DeleteUserInfo", "bot-user-id").Return(nil).Once()
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
				s.On("DeleteUserInfo", "bot-user-id").Return(errors.New("Error while disconnecting the bot account")).Once()
			},
		},
		{
			description: "Unable to disconnect the bot account due to bad permissions",
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", "", model.PermissionManageSystem).Return(false).Times(1)
				api.On("SendEphemeralPost", "", &model.Post{
					UserId:  "bot-user-id",
					Message: "Unable to disconnect the bot account, only system admins can disconnect the bot account.",
				}).Return(testutils.GetPost("", "")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p.SetAPI(mockAPI)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(p.store.(*mockStore.Store))

			_, _ = p.executeDisconnectBotCommand(testCase.args)
		})
	}
}

func TestExecuteLinkCommand(t *testing.T) {
	p := newTestPlugin(t)

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
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: model.NewString("/"),
					},
				}, nil).Times(2)
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "The MS Teams channel is now linked to this Mattermost channel.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamUserID()).Return(true).Times(1)
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetLinkByMSTeamsChannelID", testutils.GetTeamUserID(), testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
				s.On("StoreChannelLink", mock.Anything).Return(nil).Times(1)
				s.On("SaveChannelSubscription", mock.Anything).Return(nil).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {
				uc.On("GetChannelInTeam", testutils.GetTeamUserID(), testutils.GetChannelID()).Return(&msteams.Channel{}, nil)
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
					Message:   "A link for this channel already exists. Please unlink the channel before you link again with another channel.",
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
				uc.On("GetChannelInTeam", testutils.GetTeamUserID(), testutils.GetChannelID()).Return(&msteams.Channel{}, nil)
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
				api.On("LogError", "This team is not enabled for MS Teams sync.", "TeamID", "").Times(1)
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
				api.On("LogError", "Unable to get the current channel information.", "ChannelID", "", "Error", "").Times(1)
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
				api.On("LogError", "Unable to link the channel. You have to be a channel admin to link it.", "ChannelID", testutils.GetChannelID()).Times(1)
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
				api.On("LogError", "Linking/unlinking a direct or group message is not allowed.", "ChannelType", model.ChannelTypeGroup).Times(1)
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
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: model.NewString("/"),
					},
				}, nil).Times(1)
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "MS Teams channel not found or you don't have the permissions to access it.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Times(1)
				api.On("LogError", "Error occurred while getting channel in MS Teams team.", "Error", "Error while getting the channel").Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamUserID()).Return(true).Times(1)
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetLinkByMSTeamsChannelID", testutils.GetTeamUserID(), "").Return(nil, nil).Times(1)
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {
				uc.On("GetChannelInTeam", testutils.GetTeamUserID(), "").Return(nil, errors.New("Error while getting the channel"))
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			mockAPI := &plugintest.API{}
			p.SetAPI(mockAPI)
			testCase.setupAPI(mockAPI)

			testCase.setupStore(p.store.(*mockStore.Store))
			testCase.setupClient(p.msteamsAppClient.(*mockClient.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*mockClient.Client))
			_, _ = p.executeLinkCommand(testCase.args, testCase.parameters)
		})
	}
}

func TestExecuteConnectCommand(t *testing.T) {
	p := newTestPlugin(t)
	mockAPI := &plugintest.API{}

	for _, testCase := range []struct {
		description string
		setupAPI    func(*plugintest.API)
		setupStore  func(*mockStore.Store)
	}{
		{
			description: "Unable to store OAuth state",
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error in storing the OAuth state", "error", "error in storing oauth state")
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error trying to connect the account, please try again.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(errors.New("error in storing oauth state"))
			},
		},
		{
			description: "Unable to set in KV store",
			setupAPI: func(api *plugintest.API) {
				api.On("KVSet", "_code_verifier_"+testutils.GetUserID(), mock.Anything).Return(testutils.GetInternalServerAppError("unable to set in KV store")).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error trying to connect the account, please try again.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(nil)
			},
		},
		{
			description: "Successful execution of the command",
			setupAPI: func(api *plugintest.API) {
				api.On("KVSet", "_code_verifier_"+testutils.GetUserID(), mock.Anything).Return(nil).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), mock.AnythingOfType("*model.Post")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: model.NewString("/"),
					},
				}, nil).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(nil)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p.SetAPI(mockAPI)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(p.store.(*mockStore.Store))

			_, _ = p.executeConnectCommand(&model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			})
		})
	}
}

func TestExecuteConnectBotCommand(t *testing.T) {
	p := newTestPlugin(t)
	mockAPI := &plugintest.API{}

	for _, testCase := range []struct {
		description string
		setupAPI    func(*plugintest.API)
		setupStore  func(*mockStore.Store)
	}{
		{
			description: "User don't have permission to execute the command",
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(false).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Unable to connect the bot account, only system admins can connect the bot account.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(_ *mockStore.Store) {},
		},
		{
			description: "Unable to store OAuth state",
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()
				api.On("LogError", "Error in storing the OAuth state", "error", "error in storing oauth state")
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error trying to connect the bot account, please try again.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(errors.New("error in storing oauth state"))
			},
		},
		{
			description: "Unable to set in KV store",
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()
				api.On("KVSet", "_code_verifier_"+p.userID, mock.Anything).Return(testutils.GetInternalServerAppError("unable to set in KV store")).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error trying to connect the bot account, please try again.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(nil)
			},
		},
		{
			description: "Successful execution of the command",
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()
				api.On("KVSet", "_code_verifier_"+p.userID, mock.Anything).Return(nil).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), mock.AnythingOfType("*model.Post")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: model.NewString("/"),
					},
				}, nil).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(nil)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p.SetAPI(mockAPI)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(p.store.(*mockStore.Store))

			_, _ = p.executeConnectBotCommand(&model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			})
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
						Trigger:     "show-links",
						HelpText:    "Show all MS Teams linked channels",
						RoleID:      model.SystemAdminRoleId,
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
					{
						Trigger:  "promote",
						HelpText: "Promote a user from synthetic user account to regular mattermost account",
						RoleID:   model.SystemAdminRoleId,
						Arguments: []*model.AutocompleteArg{
							{
								HelpText: "Username of the existing mattermost user",
								Type:     "TextInput",
								Required: true,
								Data: &model.AutocompleteTextArg{
									Hint:    "username",
									Pattern: `^[a-z0-9\.\-_:]+$`,
								},
							},
							{
								HelpText: "The new username after the user is promoted",
								Type:     "TextInput",
								Required: true,
								Data: &model.AutocompleteTextArg{
									Hint:    "new username",
									Pattern: `^[a-z0-9\.\-_:]+$`,
								},
							},
						},
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

func TestExecutePromoteCommand(t *testing.T) {
	p := newTestPlugin(t)

	for _, testCase := range []struct {
		description string
		params      []string
		setupAPI    func(*plugintest.API)
		setupStore  func(*mockStore.Store)
	}{
		{
			description: "No params",
			params:      []string{},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Invalid promote command, please pass the current username and promoted username as parameters.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {},
		},
		{
			description: "Too many params",
			params:      []string{"user1", "user2", "user3"},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Invalid promote command, please pass the current username and promoted username as parameters.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {},
		},
		{

			description: "Not admin permissions",
			params:      []string{"valid-user", "valid-user"},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(false).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Unable to execute the command, only system admins have access to execute this command.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {},
		},
		{
			description: "Not existing user",
			params:      []string{"not-existing-user", "not-existing-user"},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("GetUserByUsername", "not-existing-user").Return(nil, &model.AppError{}).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error: Unable to promote account not-existing-user, user not found",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {},
		},
		{
			description: "Existing user but not without msteams relation",
			params:      []string{"existing-user", "existing-user"},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("GetUserByUsername", "existing-user").Return(&model.User{Id: "test", Username: "existing-user", RemoteId: model.NewString("test")}, nil).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error: Unable to promote account existing-user, it is not a known msteams user account",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "test").Return("", errors.New("not-found")).Times(1)
			},
		},
		{
			description: "Existing user, with msteams relation but without remote id",
			params:      []string{"existing-user", "existing-user"},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("GetUserByUsername", "existing-user").Return(&model.User{Id: "test", Username: "existing-user", RemoteId: nil}, nil).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error: Unable to promote account existing-user, it is already a regular account",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "test").Return("ms-test", nil).Times(1)
			},
		},
		{
			description: "Valid user, but new username is already taken",
			params:      []string{"valid-user", "new-user"},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("GetUserByUsername", "valid-user").Return(&model.User{Id: "test", Username: "valid-user", RemoteId: model.NewString("test")}, nil).Once()
				api.On("GetUserByUsername", "new-user").Return(&model.User{Id: "test2", Username: "new-user", RemoteId: nil}, nil).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error: the promoted username already exists, please use a different username.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "test").Return("ms-test", nil).Times(1)
			},
		},
		{
			description: "Valid user and valid new username, but error on update",
			params:      []string{"valid-user", "new-user"},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("GetUserByUsername", "valid-user").Return(&model.User{Id: "test", Username: "valid-user", RemoteId: model.NewString("test")}, nil).Once()
				api.On("GetUserByUsername", "new-user").Return(nil, &model.AppError{}).Once()
				api.On("UpdateUser", mock.Anything).Return(nil, &model.AppError{}).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error: Unable to promote account valid-user",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "test").Return("ms-test", nil).Times(1)
			},
		},
		{
			description: "Valid user and valid new username",
			params:      []string{"valid-user", "new-user"},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("GetUserByUsername", "valid-user").Return(&model.User{Id: "test", Username: "valid-user", RemoteId: model.NewString("test")}, nil).Once()
				api.On("GetUserByUsername", "new-user").Return(nil, &model.AppError{}).Once()
				api.On("UpdateUser", &model.User{Id: "test", Username: "new-user", RemoteId: nil}).Return(&model.User{Id: "test", Username: "new-user", RemoteId: nil}, nil).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Account valid-user has been promoted and updated the username to new-user",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "test").Return("ms-test", nil).Times(1)
			},
		},
		{
			description: "Valid user and valid new username with same username",
			params:      []string{"valid-user", "valid-user"},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("GetUserByUsername", "valid-user").Return(&model.User{Id: "test", Username: "valid-user", RemoteId: model.NewString("test")}, nil).Times(2)
				api.On("UpdateUser", &model.User{Id: "test", Username: "valid-user", RemoteId: nil}).Return(&model.User{Id: "test", Username: "valid-user", RemoteId: nil}, nil).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Account valid-user has been promoted and updated the username to valid-user",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "test").Return("ms-test", nil).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			mockAPI := &plugintest.API{}

			p.SetAPI(mockAPI)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(p.store.(*mockStore.Store))

			_, _ = p.executePromoteUserCommand(&model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			}, testCase.params)
		})
	}
}
