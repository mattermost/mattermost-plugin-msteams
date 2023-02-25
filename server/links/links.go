package links

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

const (
	channelsLinkedPrefix = "channelsLinked_"
	subscriptionPrefix   = "subscription_"
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

	webhookSecret       string
	notificationURL     string
	enabledTeams        string
	subscriptionID      string
	chatsSubscriptionID string
}

func New(api plugin.API, getMsteamsAppClient func() msteams.Client) *LinksService {
	return &LinksService{
		stopSubscriptions:   func() {},
		api:                 api,
		getMsteamsAppClient: getMsteamsAppClient,
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

	keys, appErr := ls.api.KVList(0, 1000000000)
	if appErr != nil {
		return appErr
	}

	links := []ChannelLink{}
	for _, key := range keys {
		if strings.HasPrefix(key, subscriptionPrefix) {
			appErr := ls.api.KVDelete(key)
			if appErr != nil {
				return appErr
			}
		}

		if strings.HasPrefix(key, channelsLinkedPrefix) {
			linkdata, appErr := ls.api.KVGet(key)
			if appErr != nil {
				return appErr
			}
			var link ChannelLink
			if err := json.Unmarshal(linkdata, &link); err != nil {
				return err
			}
			links = append(links, link)
		}
	}

	subscriptionID, err := ls.getMsteamsAppClient().Subscribe(ls.notificationURL, ls.webhookSecret)
	if err != nil {
		ls.api.LogError("Unable to subscribe to channels", "error", err)
		return err
	}
	ls.subscriptionID = subscriptionID

	chatsSubscriptionID, err := ls.getMsteamsAppClient().SubscribeToChats(ls.notificationURL, ls.webhookSecret)
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

func (ls *LinksService) GetLinkByChannelID(channelID string) *ChannelLink {
	data, appErr := ls.api.KVGet(channelsLinkedPrefix + channelID)
	if appErr != nil || len(data) == 0 {
		return nil
	}

	var link ChannelLink
	err := json.Unmarshal(data, &link)
	if err != nil {
		ls.api.LogError("Error getting channel link", "error", err)
		return nil
	}

	if !ls.checkEnabledTeamByTeamId(link.MattermostTeam) {
		return nil
	}
	return &link
}

func (ls *LinksService) checkEnabledTeamByTeamId(teamId string) bool {
	if ls.enabledTeams == "" {
		return true
	}
	team, appErr := ls.api.GetTeam(teamId)
	if appErr != nil {
		return false
	}
	isTeamEnabled := false
	enabledTeams := strings.Split(ls.enabledTeams, ",")
	for _, enabledTeam := range enabledTeams {
		if team.Name == enabledTeam {
			isTeamEnabled = true
			break
		}
	}
	return isTeamEnabled
}

func (ls *LinksService) DeleteLinkByChannelId(channelID string) error {
	data, appErr := ls.api.KVGet(channelsLinkedPrefix + channelID)
	if appErr != nil || len(data) == 0 {
		return errors.New("link doesn't exist")
	}

	var link ChannelLink
	err := json.Unmarshal(data, &link)
	if err != nil {
		return err
	}

	appErr = ls.api.KVDelete(channelsLinkedPrefix + channelID)
	if appErr != nil {
		return appErr
	}

	return nil
}

func (ls *LinksService) AddLink(link *ChannelLink) error {
	linkdata, err := json.Marshal(&link)
	if err != nil {
		return err
	}
	appErr := ls.api.KVSet(channelsLinkedPrefix+link.MattermostChannel, linkdata)
	if appErr != nil {
		ls.api.LogError("Unable to store channel link", "error", appErr)
		return appErr
	}
	return nil

}

func (ls *LinksService) UpdateNotificationURL(notificationURL string) {
	ls.notificationURL = notificationURL
}

func (ls *LinksService) UpdateWebhookSecret(webhookSecret string) {
	ls.webhookSecret = webhookSecret
}

func (ls *LinksService) UpdateEnabledTeams(enabledTeams string) {
	ls.enabledTeams = enabledTeams
}
