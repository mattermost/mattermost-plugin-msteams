package main

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

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
