package links

import (
	"context"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

type ChannelLink struct {
	MattermostTeam    string
	MattermostChannel string
	MSTeamsTeam       string
	MSTeamsChannel    string
	SubscriptionID    string
}

type LinksService struct {
	stopSubscriptions func()

	stopContext         context.Context
	api                 plugin.API
	getMsteamsAppClient func() msteams.Client

	webhookSecret       func() string
	notificationURL     func() string
	subscriptionID      string
	chatsSubscriptionID string
}

func New(
	api plugin.API,
	getMsteamsAppClient func() msteams.Client,
	getWebhookSecret func() string,
	getNotificationURL func() string,
) *LinksService {
	return &LinksService{
		stopSubscriptions:   func() {},
		api:                 api,
		getMsteamsAppClient: getMsteamsAppClient,
		webhookSecret:       getWebhookSecret,
		notificationURL:     getNotificationURL,
	}
}

func (ls *LinksService) Stop() {
	if ls.stopSubscriptions != nil {
		ls.stopSubscriptions()
	}
}

func (ls *LinksService) Start() error {
	ctx, stop := context.WithCancel(context.Background())
	ls.stopSubscriptions = stop
	ls.stopContext = ctx

	err := ls.getMsteamsAppClient().ClearSubscriptions()
	if err != nil {
		ls.api.LogError("Unable to clear all subscriptions", "error", err)
	}

	subscriptionID, err := ls.getMsteamsAppClient().Subscribe(ls.notificationURL(), ls.webhookSecret())
	if err != nil {
		ls.api.LogError("Unable to subscribe to channels", "error", err)
		return err
	}
	ls.subscriptionID = subscriptionID

	chatsSubscriptionID, err := ls.getMsteamsAppClient().SubscribeToChats(ls.notificationURL(), ls.webhookSecret())
	if err != nil {
		ls.api.LogError("Unable to subscribe to chats", "error", err)
		return err
	}
	ls.subscriptionID = subscriptionID
	ls.chatsSubscriptionID = chatsSubscriptionID

	go ls.getMsteamsAppClient().RefreshSubscriptionPeriodically(ctx, subscriptionID)
	go ls.getMsteamsAppClient().RefreshSubscriptionPeriodically(ctx, chatsSubscriptionID)

	return nil
}
