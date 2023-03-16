package main

import (
	"net/http"
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	storemocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"
)

const (
	channelsLinkedPrefix = "channelsLinked_"
)

func newTestPlugin() *Plugin {
	plugin := &Plugin{
		MattermostPlugin: plugin.MattermostPlugin{
			API:    &plugintest.API{},
			Driver: &plugintest.Driver{},
		},
		configuration: &configuration{
			TenantId:      "",
			ClientId:      "",
			ClientSecret:  "",
			BotUsername:   "",
			BotPassword:   "",
			WebhookSecret: "webhooksecret",
			EncryptionKey: "encryptionkey",
		},
		msteamsAppClient: &mocks.Client{},
		msteamsBotClient: &mocks.Client{},
		store:            &storemocks.Store{},
	}
	plugin.msteamsAppClient.(*mocks.Client).On("Connect").Return(nil)
	plugin.msteamsBotClient.(*mocks.Client).On("Connect").Return(nil)
	plugin.msteamsAppClient.(*mocks.Client).On("ClearSubscriptions").Return(nil)
	plugin.msteamsAppClient.(*mocks.Client).On("RefreshSubscriptionsPeriodically", mock.Anything, mock.Anything).Return(nil)
	bot := &model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: "Created by the MS Teams Sync plugin.",
	}
	config := model.Config{}
	config.SetDefaults()
	plugin.API.(*plugintest.API).On("GetConfig").Return(&config)
	plugin.API.(*plugintest.API).On("Conn", true).Return("connection-id", nil)
	plugin.API.(*plugintest.API).On("GetUnsanitizedConfig").Return(&config)
	plugin.API.(*plugintest.API).On("SavePluginConfig", mock.Anything).Return(nil)
	plugin.API.(*plugintest.API).On("EnsureBotUser", bot).Return("bot-user-id", nil).Times(1)
	plugin.API.(*plugintest.API).On("RegisterCommand", mock.Anything).Return(nil).Times(1)
	plugin.API.(*plugintest.API).On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	plugin.API.(*plugintest.API).On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	plugin.API.(*plugintest.API).On("LogWarn", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	plugin.API.(*plugintest.API).On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	plugin.API.(*plugintest.API).On("KVList", 0, 1000000000).Return([]string{}, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_subscriptions_cluster_mutex", []byte{0x1}, model.PluginKVSetOptions{Atomic: true, ExpireInSeconds: 15}).Return(true, nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithOptions", "mutex_subscriptions_cluster_mutex", []byte(nil), model.PluginKVSetOptions{Atomic: false, ExpireInSeconds: 0}).Return(true, nil).Times(1)

	plugin.OnActivate()
	return plugin
}

func TestMessageHasBeenPostedNewMessage(t *testing.T) {
	plugin := newTestPlugin()

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

	link := store.ChannelLink{
		MattermostTeam:    "team-id",
		MattermostChannel: "channel-id",
		MSTeamsTeam:       "ms-team-id",
		MSTeamsChannel:    "ms-channel-id",
	}
	plugin.store.(*storemocks.Store).On("GetLinkByChannelID", "channel-id").Return(&link, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetChannel", "channel-id").Return(&channel, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetUser", "user-id").Return(&model.User{Id: "user-id", Username: "test-user"}, nil).Times(1)
	plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "user-id").Return(&oauth2.Token{}, nil).Times(1)
	plugin.msteamsBotClient.(*mocks.Client).On("SendMessageWithAttachments", "ms-team-id", "ms-channel-id", "", "test-user@mattermost: message", []*msteams.Attachment(nil)).Return("new-message-id", nil).Times(1)
	plugin.store.(*storemocks.Store).On("LinkPosts", "post-id", "new-message-id").Return(nil).Times(1)

	plugin.MessageHasBeenPosted(nil, &post)
}

func TestMessageHasBeenPostedNewMessageWithoutChannelLink(t *testing.T) {
	plugin := newTestPlugin()

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
	plugin.store.(*storemocks.Store).On("GetLinkByChannelID", "channel-id").Return(nil, model.NewAppError("test", "not-found", nil, "", http.StatusNotFound)).Times(1)
	plugin.MessageHasBeenPosted(nil, &post)
}

func TestMessageHasBeenPostedNewMessageWithFailureSending(t *testing.T) {
	plugin := newTestPlugin()

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

	link := store.ChannelLink{
		MattermostTeam:    "team-id",
		MattermostChannel: "channel-id",
		MSTeamsTeam:       "ms-team-id",
		MSTeamsChannel:    "ms-channel-id",
	}
	plugin.store.(*storemocks.Store).On("GetLinkByChannelID", "channel-id").Return(&link, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetChannel", "channel-id").Return(&channel, nil).Times(1)
	plugin.API.(*plugintest.API).On("GetUser", "user-id").Return(&model.User{Id: "user-id", Username: "test-user"}, nil).Times(1)
	plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "user-id").Return(&oauth2.Token{}, nil).Times(1)
	plugin.msteamsBotClient.(*mocks.Client).On("SendMessageWithAttachments", "ms-team-id", "ms-channel-id", "", "test-user@mattermost: message", []*msteams.Attachment(nil)).Return("", errors.New("test-error")).Times(1)

	plugin.MessageHasBeenPosted(nil, &post)
}
