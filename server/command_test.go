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
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManagePublicChannelProperties).Return(true).Times(1)
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
				api.On("HasPermissionToChannel", testutils.GetUserID(), "Mock-ChannelID", model.PermissionManagePublicChannelProperties).Return(true).Times(1)
			},
			setupStore: func(s *mockStore.Store) {
				s.On("DeleteLinkByChannelID", "Mock-ChannelID").Return(errors.New("Error while deleting a link")).Times(1)
			},
		},
		{
			description: "Unable to get the current channel",
			args:        &model.CommandArgs{},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", "").Return(nil, testutils.GetInternalServerAppError("Error while getting the current channel.")).Times(1)
			},
			setupStore: func(s *mockStore.Store) {},
		},
		{
			description: "Unable to unlink channel as user is not a channel admin.",
			args: &model.CommandArgs{
				ChannelId: testutils.GetChannelID(),
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Id:   testutils.GetChannelID(),
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("HasPermissionToChannel", "", testutils.GetChannelID(), model.PermissionManagePublicChannelProperties).Return(false).Times(1)
			},
			setupStore: func(s *mockStore.Store) {},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			mockAPI.On("SendEphemeralPost", mock.Anything, mock.Anything).Return(testutils.GetPost())
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
			setupAPI: func(api *plugintest.API) {},
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
			setupAPI:    func(api *plugintest.API) {},
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
			setupAPI: func(api *plugintest.API) {},
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
			mockAPI.On("SendEphemeralPost", mock.Anything, mock.Anything).Return(testutils.GetPost())
			testCase.setupAPI(mockAPI)
			p.SetAPI(mockAPI)

			testCase.setupStore(p.store.(*mockStore.Store))
			testCase.setupClient(p.msteamsAppClient.(*mockClient.Client))
			_, _ = p.executeShowCommand(&plugin.Context{}, testCase.args)
		})
	}
}
