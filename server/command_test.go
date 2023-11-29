package main

import (
	"database/sql"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	mockMetrics "github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	mockClient "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	mockStore "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
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
		setupClient func(*mockClient.Client)
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "The MS Teams channel is no longer linked to this Mattermost channel.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
				api.On("LogDebug", "Unable to delete the subscription on MS Teams", "subscriptionID", "testSubscriptionID", "error", "unable to delete the subscription").Return().Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MSTeamsChannel: "Valid-MSTeamsChannel",
				}, nil).Once()
				s.On("DeleteLinkByChannelID", testutils.GetChannelID()).Return(nil).Times(1)
				s.On("GetChannelSubscriptionByTeamsChannelID", "Valid-MSTeamsChannel").Return(&storemodels.ChannelSubscription{
					SubscriptionID: "testSubscriptionID",
				}, nil).Once()
				s.On("DeleteSubscription", "testSubscriptionID").Return(nil).Once()
			},
			setupClient: func(c *mockClient.Client) {
				c.On("DeleteSubscription", "testSubscriptionID").Return(errors.New("unable to delete the subscription")).Once()
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", "Mock-ChannelID", "This Mattermost channel is not linked to any MS Teams channel.")).Return(testutils.GetPost("Mock-ChannelID", testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
				api.On("LogDebug", "Unable to get the link by channel ID", "error", "Error while getting link").Return().Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", "Mock-ChannelID").Return(nil, errors.New("Error while getting link")).Once()
			},
			setupClient: func(c *mockClient.Client) {},
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", "Mock-ChannelID", "Unable to delete link.")).Return(testutils.GetPost("Mock-ChannelID", testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
				api.On("LogDebug", "Unable to delete the link by channel ID", "error", "Error while deleting a link").Return().Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", "Mock-ChannelID").Return(nil, nil).Once()
				s.On("DeleteLinkByChannelID", "Mock-ChannelID").Return(errors.New("Error while deleting a link")).Times(1)
			},
			setupClient: func(c *mockClient.Client) {},
		},
		{
			description: "Unable to get the current channel",
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", "").Return(nil, testutils.GetInternalServerAppError("Error while getting the current channel.")).Once()
				api.On("SendEphemeralPost", "", testutils.GetEphemeralPost("bot-user-id", "", "Unable to get the current channel information.")).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id", time.Now().UnixMicro())).Times(1)
			},
			setupStore:  func(s *mockStore.Store) {},
			setupClient: func(c *mockClient.Client) {},
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Unable to unlink the channel, you have to be a channel admin to unlink it.")).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id", time.Now().UnixMicro())).Times(1)
			},
			setupStore:  func(s *mockStore.Store) {},
			setupClient: func(c *mockClient.Client) {},
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Linking/unlinking a direct or group message is not allowed")).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id", time.Now().UnixMicro())).Times(1)
			},
			setupStore:  func(s *mockStore.Store) {},
			setupClient: func(c *mockClient.Client) {},
		},
		{
			description: "Unable to get subscription by Teams channel ID.",
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "The MS Teams channel is no longer linked to this Mattermost channel.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
				api.On("LogDebug", "Unable to get the subscription by MS Teams channel ID", "error", "unable to get the subscription").Return().Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MSTeamsChannel: "Valid-MSTeamsChannel",
				}, nil).Once()
				s.On("DeleteLinkByChannelID", testutils.GetChannelID()).Return(nil).Times(1)
				s.On("GetChannelSubscriptionByTeamsChannelID", "Valid-MSTeamsChannel").Return(nil, errors.New("unable to get the subscription")).Once()
			},
			setupClient: func(c *mockClient.Client) {},
		},
		{
			description: "Unable to delete the subscription from the DB",
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "The MS Teams channel is no longer linked to this Mattermost channel.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
				api.On("LogDebug", "Unable to delete the subscription from the DB", "subscriptionID", "testSubscriptionID", "error", "unable to delete the subscription").Return().Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MSTeamsChannel: "Valid-MSTeamsChannel",
				}, nil).Once()
				s.On("DeleteLinkByChannelID", testutils.GetChannelID()).Return(nil).Times(1)
				s.On("GetChannelSubscriptionByTeamsChannelID", "Valid-MSTeamsChannel").Return(&storemodels.ChannelSubscription{
					SubscriptionID: "testSubscriptionID",
				}, nil).Once()
				s.On("DeleteSubscription", "testSubscriptionID").Return(errors.New("unable to delete the subscription")).Once()
			},
			setupClient: func(c *mockClient.Client) {},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			testCase.setupAPI(mockAPI)
			p.SetAPI(mockAPI)
			defer mockAPI.AssertExpectations(t)
			testCase.setupStore(p.store.(*mockStore.Store))
			testCase.setupClient(p.msteamsAppClient.(*mockClient.Client))
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "This channel is linked to the MS Teams Channel \"\" in the Team \"\".")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MSTeamsTeam: "Valid-MSTeamsTeam",
				}, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {
				c.On("GetTeam", "Valid-MSTeamsTeam").Return(&clientmodels.Team{}, nil).Times(1)
				c.On("GetChannelInTeam", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&clientmodels.Channel{}, nil).Times(1)
			},
		},
		{
			description: "Unable to get the link",
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", "", testutils.GetEphemeralPost("bot-user-id", "", "Link doesn't exist.")).Return(testutils.GetPost("", "", time.Now().UnixMicro())).Times(1)
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
				api.On("SendEphemeralPost", "", testutils.GetEphemeralPost("bot-user-id", "Invalid-ChannelID", "Unable to get the MS Teams team information.")).Return(testutils.GetPost("Invalid-ChannelID", "", time.Now().UnixMicro())).Times(1)
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

				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), commandWaitingMessage)).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()

				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "| Mattermost Team | Mattermost Channel | MS Teams Team | MS Teams Channel | \n| :------|:--------|:-------|:-----------|\n|Test MM team|Test MM channel|Test MS team|Test MS channel|")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("ListChannelLinksWithNames").Return(testutils.GetChannelLinks(1), nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {
				c.On("GetTeams", mock.AnythingOfType("string")).Return([]*clientmodels.Team{{ID: testutils.GetTeamsTeamID(), DisplayName: "Test MS team"}}, nil).Times(1)
				c.On("GetChannelsInTeam", testutils.GetTeamsTeamID(), mock.AnythingOfType("string")).Return([]*clientmodels.Channel{{ID: testutils.GetTeamsChannelID(), DisplayName: "Test MS channel"}}, nil).Times(1)
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Unable to execute the command, only system admins have access to execute this command.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Something went wrong.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "No links present.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
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

				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), commandWaitingMessage)).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()

				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "| Mattermost Team | Mattermost Channel | MS Teams Team | MS Teams Channel | \n| :------|:--------|:-------|:-----------|\n|Test MM team|Test MM channel|||\n|Test MM team|Test MM channel|||\n|Test MM team|Test MM channel|||\n|Test MM team|Test MM channel|||\nThere were some errors while fetching information. Please check the server logs.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
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

func TestExecuteSyncCommand(t *testing.T) {
	for _, testCase := range []struct {
		description string
		params      []string
		args        *model.CommandArgs
		setupAPI    func(*plugintest.API)
		setupStore  func(*mockStore.Store)
		setupClient func(*mockClient.Client)
	}{
		{
			description: "Successfully executed sync command",
			params:      []string{},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Synchronizing last 24 hours of the channel...")).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id", time.Now().UnixMicro())).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Synchronization complete.")).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id", time.Now().UnixMicro())).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Id:   testutils.GetChannelID(),
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MSTeamsTeam:    "Valid-MSTeamsTeam",
					MSTeamsChannel: "Valid-MSTeamsChannel",
				}, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {
				c.On("GetTeam", "Valid-MSTeamsTeam").Return(&clientmodels.Team{}, nil).Times(1)
				c.On("GetChannelInTeam", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&clientmodels.Channel{}, nil).Times(1)
				c.On("ListChannelMessages", "Valid-MSTeamsTeam", "Valid-MSTeamsChannel", mock.AnythingOfType("time.Time")).Return([]*clientmodels.Message{}, nil).Times(1)
			},
		},
		{
			description: "Successfully executed sync command with defined number of hours",
			params:      []string{"8"},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Synchronizing last 8 hours of the channel...")).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id", time.Now().UnixMicro())).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Synchronization complete.")).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id", time.Now().UnixMicro())).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Id:   testutils.GetChannelID(),
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MSTeamsTeam:    "Valid-MSTeamsTeam",
					MSTeamsChannel: "Valid-MSTeamsChannel",
				}, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {
				c.On("GetTeam", "Valid-MSTeamsTeam").Return(&clientmodels.Team{}, nil).Times(1)
				c.On("GetChannelInTeam", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&clientmodels.Channel{}, nil).Times(1)
				c.On("ListChannelMessages", "Valid-MSTeamsTeam", "Valid-MSTeamsChannel", mock.AnythingOfType("time.Time")).Return([]*clientmodels.Message{}, nil).Times(1)
			},
		},
		{
			description: "Unable to get the link",
			params:      []string{},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Link doesn't exist.")).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id", time.Now().UnixMicro())).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Id:   testutils.GetChannelID(),
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, errors.New("Error while getting the link")).Times(1)
			},
			setupClient: func(c *mockClient.Client) {},
		},
		{
			description: "Unable to get the MS Teams team information",
			params:      []string{},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Unable to get the MS Teams team information.")).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id", time.Now().UnixMicro())).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Id:   testutils.GetChannelID(),
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MSTeamsTeam: "Invalid-MSTeamsTeam",
				}, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {
				c.On("GetTeam", "Invalid-MSTeamsTeam").Return(nil, errors.New("Error while getting the MS Teams team information")).Times(1)
			},
		},
		{
			description: "Unable to get the MS Teams channel messages",
			params:      []string{},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Synchronizing last 24 hours of the channel...")).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id", time.Now().UnixMicro())).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Synchronization failed.")).Return(testutils.GetPost(testutils.GetChannelID(), "bot-user-id", time.Now().UnixMicro())).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Id:   testutils.GetChannelID(),
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)

				api.On("LogError", "Unable to sync channel messages", "teamID", "Valid-MSTeamsTeam", "channelID", "Valid-MSTeamsChannel", "synce", mock.AnythingOfType("time.Time"), "error", mock.Anything).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MSTeamsTeam:    "Valid-MSTeamsTeam",
					MSTeamsChannel: "Valid-MSTeamsChannel",
				}, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client) {
				c.On("GetTeam", "Valid-MSTeamsTeam").Return(&clientmodels.Team{}, nil).Times(1)
				c.On("GetChannelInTeam", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&clientmodels.Channel{}, nil).Times(1)
				c.On("ListChannelMessages", "Valid-MSTeamsTeam", "Valid-MSTeamsChannel", mock.AnythingOfType("time.Time")).Return(nil, errors.New("unable to get channel messages")).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			mockAPI := &plugintest.API{}

			testCase.setupAPI(mockAPI)
			p.SetAPI(mockAPI)

			testCase.setupStore(p.store.(*mockStore.Store))
			testCase.setupClient(p.msteamsAppClient.(*mockClient.Client))
			_, _ = p.executeSyncCommand(testCase.args, testCase.params)
			time.Sleep(10 * time.Millisecond)
			mockAPI.AssertExpectations(t)
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", "", "Your account has been disconnected.")).Return(testutils.GetPost("", testutils.GetUserID(), time.Now().UnixMicro())).Times(1)

				api.On("LogDebug", "Unable to delete the last prompt timestamp for the user", "MMUserID", testutils.GetUserID(), "Error", "error in deleting prompt time")
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", testutils.GetUserID()).Return(testutils.GetTeamsUserID(), nil).Times(1)
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, nil).Once()
				var token *oauth2.Token
				s.On("SetUserInfo", testutils.GetUserID(), testutils.GetTeamsUserID(), token).Return(nil).Times(1)
				s.On("DeleteDMAndGMChannelPromptTime", testutils.GetUserID()).Return(errors.New("error in deleting prompt time")).Once()
			},
		},
		{
			description: "User account is not connected",
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", "", testutils.GetEphemeralPost("bot-user-id", "", "Error: the account is not connected")).Return(testutils.GetPost("", "", time.Now().UnixMicro())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("MattermostToTeamsUserID", "").Return("", errors.New("Unable to get team UserID")).Times(1)
			},
		},
		{
			description: "User account is not connected as token is not found",
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", "", testutils.GetEphemeralPost("bot-user-id", "", "Error: the account is not connected")).Return(testutils.GetPost("", "", time.Now().UnixMicro())).Times(1)
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", "", "Error: unable to disconnect your account, Error while disconnecting your account")).Return(testutils.GetPost("", testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "The bot account has been disconnected.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Error: unable to find the connected bot account")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Error: unable to disconnect the bot account, Error while disconnecting the bot account")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
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
				api.On("SendEphemeralPost", "", testutils.GetEphemeralPost("bot-user-id", "", "Unable to disconnect the bot account, only system admins can disconnect the bot account.")).Return(testutils.GetPost("", "", time.Now().UnixMicro())).Times(1)
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
	mockAPI := &plugintest.API{}

	for _, testCase := range []struct {
		description  string
		parameters   []string
		args         *model.CommandArgs
		setupAPI     func(*plugintest.API)
		setupStore   func(*mockStore.Store)
		setupClient  func(*mockClient.Client, *mockClient.Client)
		setupMetrics func(mockmetrics *mockMetrics.Metrics)
	}{
		{
			description: "Successfully executed link command",
			parameters:  []string{testutils.GetTeamsUserID(), testutils.GetChannelID()},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				TeamId:    testutils.GetTeamsUserID(),
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), commandWaitingMessage)).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "The MS Teams channel is now linked to this Mattermost channel.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamsUserID()).Return(true).Times(1)
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetLinkByMSTeamsChannelID", testutils.GetTeamsUserID(), testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
				s.On("StoreChannelLink", mock.AnythingOfType("*storemodels.ChannelLink")).Return(nil).Times(1)
				s.On("BeginTx").Return(&sql.Tx{}, nil).Times(1)
				s.On("SaveChannelSubscription", &sql.Tx{}, mock.AnythingOfType("storemodels.ChannelSubscription")).Return(nil).Times(1)
				s.On("CommitTx", &sql.Tx{}).Return(nil).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {
				uc.On("GetChannelInTeam", testutils.GetTeamsUserID(), testutils.GetChannelID()).Return(&clientmodels.Channel{}, nil)
			},
			setupMetrics: func(mockmetrics *mockMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionConnected).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChannelInTeam", "true", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Error in beginning the database transaction",
			parameters:  []string{testutils.GetTeamsUserID(), testutils.GetChannelID()},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				TeamId:    testutils.GetTeamsUserID(),
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), commandWaitingMessage)).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Something went wrong")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
				api.On("LogError", "Unable to begin the database transaction", "error", "error in beginning the database transaction")
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamsUserID()).Return(true).Times(1)
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetLinkByMSTeamsChannelID", testutils.GetTeamsUserID(), testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
				s.On("StoreChannelLink", mock.AnythingOfType("*storemodels.ChannelLink")).Return(nil).Times(1)
				s.On("BeginTx").Return(nil, errors.New("error in beginning the database transaction")).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {
				uc.On("GetChannelInTeam", testutils.GetTeamsUserID(), testutils.GetChannelID()).Return(&clientmodels.Channel{}, nil)
			},
			setupMetrics: func(mockmetrics *mockMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionConnected).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChannelInTeam", "true", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Unable to commit the database transaction",
			parameters:  []string{testutils.GetTeamsUserID(), testutils.GetChannelID()},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				TeamId:    testutils.GetTeamsUserID(),
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), commandWaitingMessage)).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Something went wrong")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
				api.On("LogError", "Unable to commit database transaction", "error", "error in committing transaction")
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamsUserID()).Return(true).Times(1)
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetLinkByMSTeamsChannelID", testutils.GetTeamsUserID(), testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
				s.On("StoreChannelLink", mock.AnythingOfType("*storemodels.ChannelLink")).Return(nil).Times(1)
				s.On("BeginTx").Return(&sql.Tx{}, nil).Times(1)
				s.On("SaveChannelSubscription", &sql.Tx{}, mock.AnythingOfType("storemodels.ChannelSubscription")).Return(nil).Times(1)
				s.On("CommitTx", &sql.Tx{}).Return(errors.New("error in committing transaction")).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {
				uc.On("GetChannelInTeam", testutils.GetTeamsUserID(), testutils.GetChannelID()).Return(&clientmodels.Channel{}, nil)
			},
			setupMetrics: func(mockmetrics *mockMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionConnected).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChannelInTeam", "true", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Unable to link a MS Teams channel to multiple channels",
			parameters:  []string{testutils.GetTeamsUserID(), testutils.GetChannelID()},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				TeamId:    testutils.GetTeamsUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "A link for this channel already exists. Please unlink the channel before you link again with another channel.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamsUserID()).Return(true).Times(1)
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetLinkByMSTeamsChannelID", testutils.GetTeamsUserID(), testutils.GetChannelID()).Return(&storemodels.ChannelLink{}, nil).Times(1)
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
				s.On("StoreChannelLink", mock.Anything).Return(nil).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {
				uc.On("GetChannelInTeam", testutils.GetTeamsUserID(), testutils.GetChannelID()).Return(&clientmodels.Channel{}, nil)
			},
			setupMetrics: func(metrics *mockMetrics.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChannelInTeam", "true", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Invalid link command",
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				TeamId:    testutils.GetTeamsUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(true).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Invalid link command, please pass the MS Teams team id and channel id as parameters.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
			},
			setupStore:   func(s *mockStore.Store) {},
			setupClient:  func(c *mockClient.Client, uc *mockClient.Client) {},
			setupMetrics: func(mockmetrics *mockMetrics.Metrics) {},
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
				api.On("SendEphemeralPost", "", testutils.GetEphemeralPost("bot-user-id", "", "This team is not enabled for MS Teams sync.")).Return(testutils.GetPost("", "", time.Now().UnixMicro())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", "").Return(false).Times(1)
			},
			setupClient:  func(c *mockClient.Client, uc *mockClient.Client) {},
			setupMetrics: func(mockmetrics *mockMetrics.Metrics) {},
		},
		{
			description: "Unable to get the current channel information",
			parameters:  []string{testutils.GetTeamsUserID(), ""},
			args: &model.CommandArgs{
				TeamId: testutils.GetTeamsUserID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", "").Return(nil, testutils.GetInternalServerAppError("Error while getting the current channel.")).Times(1)
				api.On("SendEphemeralPost", "", testutils.GetEphemeralPost("bot-user-id", "", "Unable to get the current channel information.")).Return(testutils.GetPost("", "", time.Now().UnixMicro())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamsUserID()).Return(true).Times(1)
			},
			setupClient:  func(c *mockClient.Client, uc *mockClient.Client) {},
			setupMetrics: func(mockmetrics *mockMetrics.Metrics) {},
		},
		{
			description: "Unable to link the channel as only channel admin can link it",
			parameters:  []string{testutils.GetTeamsUserID(), ""},
			args: &model.CommandArgs{
				TeamId:    testutils.GetTeamsUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", "", testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(false).Times(1)
				api.On("SendEphemeralPost", "", testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Unable to link the channel. You have to be a channel admin to link it.")).Return(testutils.GetPost(testutils.GetChannelID(), "", time.Now().UnixMicro())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamsUserID()).Return(true).Times(1)
			},
			setupClient:  func(c *mockClient.Client, uc *mockClient.Client) {},
			setupMetrics: func(mockmetrics *mockMetrics.Metrics) {},
		},
		{
			description: "Unable to link channel as channel is either a direct or group message",
			parameters:  []string{testutils.GetTeamsUserID(), ""},
			args: &model.CommandArgs{
				TeamId:    testutils.GetTeamsUserID(),
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Type: model.ChannelTypeGroup,
				}, nil).Times(1)
				api.On("SendEphemeralPost", "", testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "Linking/unlinking a direct or group message is not allowed")).Return(testutils.GetPost(testutils.GetChannelID(), "", time.Now().UnixMicro())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamsUserID()).Return(true).Times(1)
			},
			setupClient:  func(c *mockClient.Client, uc *mockClient.Client) {},
			setupMetrics: func(mockmetrics *mockMetrics.Metrics) {},
		},
		{
			description: "Unable to find MS Teams channel as user don't have the permissions to access it",
			parameters:  []string{testutils.GetTeamsUserID(), ""},
			args: &model.CommandArgs{
				UserId:    testutils.GetUserID(),
				TeamId:    testutils.GetTeamsUserID(),
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost("bot-user-id", testutils.GetChannelID(), "MS Teams channel not found or you don't have the permissions to access it.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("CheckEnabledTeamByTeamID", testutils.GetTeamsUserID()).Return(true).Times(1)
				s.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				s.On("GetLinkByMSTeamsChannelID", testutils.GetTeamsUserID(), "").Return(nil, nil).Times(1)
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			setupClient: func(c *mockClient.Client, uc *mockClient.Client) {
				uc.On("GetChannelInTeam", testutils.GetTeamsUserID(), "").Return(nil, errors.New("Error while getting the channel"))
			},
			setupMetrics: func(metrics *mockMetrics.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChannelInTeam", "false", mock.AnythingOfType("float64")).Once()
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p.SetAPI(mockAPI)
			testCase.setupAPI(mockAPI)

			testCase.setupStore(p.store.(*mockStore.Store))
			testCase.setupClient(p.msteamsAppClient.(*mockClient.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*mockClient.Client))
			testCase.setupMetrics(p.metricsService.(*mockMetrics.Metrics))
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
			description: "User already connected",
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "You are already connected to MS Teams. Please disconnect your account first before connecting again.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Once()
			},
		},
		{
			description: "Error in checking if the user is present in whitelist",
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error in checking if a user is present in whitelist", "UserID", testutils.GetUserID(), "Error", "error in accessing whitelist").Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error in trying to connect the account, please try again.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, nil).Once()
				s.On("IsUserPresentInWhitelist", testutils.GetUserID()).Return(false, errors.New("error in accessing whitelist")).Once()
			},
		},
		{
			description: "Error in getting the size of whitelist",
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error in getting the size of whitelist", "Error", "unable to get size of whitelist").Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error in trying to connect the account, please try again.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, nil).Once()
				s.On("IsUserPresentInWhitelist", testutils.GetUserID()).Return(false, nil).Once()
				s.On("GetSizeOfWhitelist").Return(0, errors.New("unable to get size of whitelist")).Once()
			},
		},
		{
			description: "Size of whitelist has reached maximum limit",
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "You cannot connect your account because the maximum limit of users allowed to connect has been reached. Please contact your system administrator.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
				api.On("KVSetWithOptions", "mutex_whitelist_cluster_mutex", []byte(nil), model.PluginKVSetOptions{ExpireInSeconds: 0}).Return(true, nil).Times(1)
				api.On("KVSetWithOptions", "mutex_whitelist_cluster_mutex", []byte{0x1}, model.PluginKVSetOptions{Atomic: true, ExpireInSeconds: 15}).Return(true, nil).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, nil).Once()
				s.On("IsUserPresentInWhitelist", testutils.GetUserID()).Return(false, nil).Once()
				s.On("GetSizeOfWhitelist").Return(0, nil).Once()
			},
		},
		{
			description: "Unable to store OAuth state",
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error in storing the OAuth state", "error", "error in storing oauth state")
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Error in trying to connect the account, please try again.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, errors.New("token not found")).Once()
				s.On("IsUserPresentInWhitelist", testutils.GetUserID()).Return(true, nil).Once()
				s.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(errors.New("error in storing oauth state"))
			},
		},
		{
			description: "Unable to set in KV store",
			setupAPI: func(api *plugintest.API) {
				api.On("KVSet", "_code_verifier_"+testutils.GetUserID(), mock.Anything).Return(testutils.GetInternalServerAppError("unable to set in KV store")).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Error in trying to connect the account, please try again.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
				api.On("KVSetWithOptions", "mutex_whitelist_cluster_mutex", []byte(nil), model.PluginKVSetOptions{ExpireInSeconds: 0}).Return(true, nil).Times(1)
				api.On("KVSetWithOptions", "mutex_whitelist_cluster_mutex", []byte{0x1}, model.PluginKVSetOptions{Atomic: true, ExpireInSeconds: 15}).Return(true, nil).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, errors.New("token not found")).Once()
				s.On("IsUserPresentInWhitelist", testutils.GetUserID()).Return(true, nil).Once()
				s.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(nil)
			},
		},
		{
			description: "Successful execution of the command",
			setupAPI: func(api *plugintest.API) {
				api.On("KVSet", "_code_verifier_"+testutils.GetUserID(), mock.Anything).Return(nil).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), mock.AnythingOfType("*model.Post")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: model.NewString("/"),
					},
				}, nil).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, errors.New("token not found")).Once()
				s.On("IsUserPresentInWhitelist", testutils.GetUserID()).Return(true, nil).Once()
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Unable to connect the bot account, only system admins can connect the bot account.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(_ *mockStore.Store) {},
		},
		{
			description: "Bot user already connected",
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "The bot account is already connected to MS Teams. Please disconnect the bot account first before connecting again.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", p.userID).Return(&oauth2.Token{}, nil).Once()
			},
		},
		{
			description: "Error in checking if the bot user is present in whitelist",
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()
				api.On("LogError", "Error in checking if the bot user is present in whitelist", "BotUserID", p.userID, "Error", "error in accessing whitelist").Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error in trying to connect the bot account, please try again.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", p.userID).Return(nil, nil).Once()
				s.On("IsUserPresentInWhitelist", p.userID).Return(false, errors.New("error in accessing whitelist")).Once()
			},
		},
		{
			description: "Error in getting the size of whitelist",
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()
				api.On("LogError", "Error in getting the size of whitelist", "Error", "unable to get size of whitelist").Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "Error in trying to connect the bot account, please try again.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", p.userID).Return(nil, nil).Once()
				s.On("IsUserPresentInWhitelist", p.userID).Return(false, nil).Once()
				s.On("GetSizeOfWhitelist").Return(0, errors.New("unable to get size of whitelist")).Once()
			},
		},
		{
			description: "Size of whitelist has reached maximum limit",
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    p.userID,
					ChannelId: testutils.GetChannelID(),
					Message:   "You cannot connect the bot account because the maximum limit of users allowed to connect has been reached.",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", p.userID).Return(nil, nil).Once()
				s.On("IsUserPresentInWhitelist", p.userID).Return(false, nil).Once()
				s.On("GetSizeOfWhitelist").Return(0, nil).Once()
			},
		},
		{
			description: "Unable to store OAuth state",
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()
				api.On("LogError", "Error in storing the OAuth state", "error", "error in storing oauth state")
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Error in trying to connect the bot account, please try again.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", p.userID).Return(nil, errors.New("token not found")).Once()
				s.On("IsUserPresentInWhitelist", p.userID).Return(true, nil).Once()
				s.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(errors.New("error in storing oauth state"))
			},
		},
		{
			description: "Unable to set in KV store",
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()
				api.On("KVSet", "_code_verifier_"+p.userID, mock.Anything).Return(testutils.GetInternalServerAppError("unable to set in KV store")).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Error in trying to connect the bot account, please try again.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", p.userID).Return(nil, errors.New("token not found")).Once()
				s.On("IsUserPresentInWhitelist", p.userID).Return(true, nil).Once()
				s.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(nil)
			},
		},
		{
			description: "Successful execution of the command",
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Once()
				api.On("KVSet", "_code_verifier_"+p.userID, mock.Anything).Return(nil).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), mock.AnythingOfType("*model.Post")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: model.NewString("/"),
					},
				}, nil).Once()
			},
			setupStore: func(s *mockStore.Store) {
				s.On("GetTokenForMattermostUser", p.userID).Return(nil, errors.New("token not found")).Once()
				s.On("IsUserPresentInWhitelist", p.userID).Return(true, nil).Once()
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Invalid promote command, please pass the current username and promoted username as parameters.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {},
		},
		{
			description: "Too many params",
			params:      []string{"user1", "user2", "user3"},
			setupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Invalid promote command, please pass the current username and promoted username as parameters.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {},
		},
		{

			description: "Not admin permissions",
			params:      []string{"valid-user", "valid-user"},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(false).Times(1)
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Unable to execute the command, only system admins have access to execute this command.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {},
		},
		{
			description: "Not existing user",
			params:      []string{"not-existing-user", "not-existing-user"},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("GetUserByUsername", "not-existing-user").Return(nil, &model.AppError{}).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Error: Unable to promote account not-existing-user, user not found")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
			},
			setupStore: func(s *mockStore.Store) {},
		},
		{
			description: "Existing user but not without msteams relation",
			params:      []string{"existing-user", "existing-user"},
			setupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("GetUserByUsername", "existing-user").Return(&model.User{Id: "test", Username: "existing-user", RemoteId: model.NewString("test")}, nil).Once()
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Error: Unable to promote account existing-user, it is not a known msteams user account")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Error: Unable to promote account existing-user, it is already a regular account")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Error: the promoted username already exists, please use a different username.")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Error: Unable to promote account valid-user")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Account valid-user has been promoted and updated the username to new-user")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
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
				api.On("SendEphemeralPost", testutils.GetUserID(), testutils.GetEphemeralPost(p.userID, testutils.GetChannelID(), "Account valid-user has been promoted and updated the username to valid-user")).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Once()
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
