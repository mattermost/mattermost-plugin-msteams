package main

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
)

func newTestPlugin() *Plugin {
	plugin := &Plugin{
		MattermostPlugin: plugin.MattermostPlugin{
			API:    &plugintest.API{},
			Driver: &plugintest.Driver{},
		},
		configuration: &configuration{
			TenantId:     "",
			ClientId:     "",
			ClientSecret: "",
			BotUsername:  "",
			BotPassword:  "",
		},
		msteamsAppClient: &mocks.Client{},
		msteamsBotClient: &mocks.Client{},
	}
	plugin.msteamsAppClient.(*mocks.Client).On("Connect").Return(nil)
	plugin.msteamsBotClient.(*mocks.Client).On("Connect").Return(nil)
	plugin.msteamsAppClient.(*mocks.Client).On("ClearSubscriptions").Return(nil)
	bot := &model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: "Created by the MS Teams Sync plugin.",
	}
	botUser := &model.User{
		Id:       "bot-user-id",
		Username: botUsername,
	}
	plugin.API.(*plugintest.API).On("CreateBot", bot).Return(bot, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetUserByUsername", botUsername).Return(botUser, nil).Times(1)
	plugin.API.(*plugintest.API).On("RegisterCommand", mock.Anything).Return(nil).Times(1)
	plugin.API.(*plugintest.API).On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	plugin.API.(*plugintest.API).On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	plugin.API.(*plugintest.API).On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	plugin.API.(*plugintest.API).On("KVGet", "channelsLinked").Return([]byte("{}"), nil).Times(1)
	plugin.OnActivate()
	return plugin
}

func TestMessageHasBeenPostedNewMessage(t *testing.T) {
	plugin := newTestPlugin()

	plugin.channelsLinked = map[string]ChannelLink{
		"team-id:channel-id": {
			MattermostTeam:    "team-id",
			MattermostChannel: "channel-id",
			MSTeamsTeam:       "ms-team-id",
			MSTeamsChannel:    "ms-channel-id",
		},
	}

	channel := model.Channel{
		Id:     "channel-id",
		TeamId: "team-id",
	}
	post := model.Post{
		Id:        "post-id",
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
		UserId:    "user-id",
		ChannelId: channel.Id,
		Message:   "message",
	}

	plugin.API.(*plugintest.API).On("GetChannel", "channel-id").Return(&channel, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetUser", "user-id").Return(&model.User{Id: "user-id", Username: "test-user"}, nil).Times(1)
	plugin.msteamsBotClient.(*mocks.Client).On("SendMessage", "ms-team-id", "ms-channel-id", "", "test-user@mattermost: message").Return("new-message-id", nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSet", "mattermost_teams_post-id", []byte("new-message-id")).Return(nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSet", "teams_mattermost_new-message-id", []byte("post-id")).Return(nil).Times(1)

	plugin.MessageHasBeenPosted(nil, &post)
}

func TestMessageHasBeenPostedNewMessageWithoutChannelLink(t *testing.T) {
	plugin := newTestPlugin()

	plugin.channelsLinked = map[string]ChannelLink{}

	channel := model.Channel{
		Id:     "channel-id",
		TeamId: "team-id",
	}
	post := model.Post{
		Id:        "post-id",
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
		UserId:    "user-id",
		ChannelId: channel.Id,
		Message:   "message",
	}

	plugin.API.(*plugintest.API).On("GetChannel", "channel-id").Return(&channel, nil).Times(1)
	plugin.MessageHasBeenPosted(nil, &post)
}

func TestMessageHasBeenPostedNewMessageWithFailureSending(t *testing.T) {
	plugin := newTestPlugin()

	plugin.channelsLinked = map[string]ChannelLink{
		"team-id:channel-id": {
			MattermostTeam:    "team-id",
			MattermostChannel: "channel-id",
			MSTeamsTeam:       "ms-team-id",
			MSTeamsChannel:    "ms-channel-id",
		},
	}

	channel := model.Channel{
		Id:     "channel-id",
		TeamId: "team-id",
	}
	post := model.Post{
		Id:        "post-id",
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
		UserId:    "user-id",
		ChannelId: channel.Id,
		Message:   "message",
	}

	plugin.API.(*plugintest.API).On("GetChannel", "channel-id").Return(&channel, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetUser", "user-id").Return(&model.User{Id: "user-id", Username: "test-user"}, nil).Times(1)
	plugin.msteamsBotClient.(*mocks.Client).On("SendMessage", "ms-team-id", "ms-channel-id", "", "test-user@mattermost: message").Return("", errors.New("test-error")).Times(1)

	plugin.MessageHasBeenPosted(nil, &post)
}
