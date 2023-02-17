package links

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

const (
	keyChannelsLinked = "channelsLinked"
)

type ChannelLink struct {
	MattermostTeam    string
	MattermostChannel string
	MSTeamsTeam       string
	MSTeamsChannel    string
}

type LinksService struct {
	StopSubscriptions func()

	subscriptionsToLinksMutex sync.Mutex
	subscriptionsToLinks      map[string]*ChannelLink
	channelsLinked            map[string]*ChannelLink
	stopContext               context.Context
	api                       plugin.API
	msteamsAppClient          msteams.Client

	webhookSecret   string
	notificationURL string
	enabledTeams    string
}

func New(api plugin.API, msteamsAppClient msteams.Client) *LinksService {
	return &LinksService{
		StopSubscriptions:    func() {},
		api:                  api,
		channelsLinked:       map[string]*ChannelLink{},
		subscriptionsToLinks: map[string]*ChannelLink{},
		msteamsAppClient:     msteamsAppClient,
	}
}

func (ls *LinksService) Save() error {
	channelsLinkedData, err := json.Marshal(ls.channelsLinked)
	if err != nil {
		return errors.New("unable to serialize the linked channels")
	}

	appErr := ls.api.KVSet(keyChannelsLinked, channelsLinkedData)
	if appErr != nil {
		return appErr
	}
	return nil
}

func (ls *LinksService) Load() error {
	channelsLinkedData, appErr := ls.api.KVGet(keyChannelsLinked)
	if appErr != nil {
		ls.api.LogError("Error getting the channels linked", "error", appErr)
	}

	channelsLinked := map[string]*ChannelLink{}
	if len(channelsLinkedData) > 0 {
		err := json.Unmarshal(channelsLinkedData, &channelsLinked)
		if err != nil {
			ls.api.LogError("Error getting the channels linked", "error", err)
			return err
		}
	}

	ls.channelsLinked = channelsLinked
	return nil
}

func (ls *LinksService) Stop() {
	if ls.StopSubscriptions != nil {
		ls.StopSubscriptions()
	}
}

func (ls *LinksService) Start() error {
	ls.subscriptionsToLinks = map[string]*ChannelLink{}
	ctx, stop := context.WithCancel(context.Background())
	ls.StopSubscriptions = stop
	ls.stopContext = ctx
	err := ls.msteamsAppClient.ClearSubscriptions()
	if err != nil {
		ls.api.LogError("Unable to clear all subscriptions", "error", err)
	}
	for _, link := range ls.channelsLinked {
		subscriptionID, err := ls.subscribeToChannel(link)
		if err != nil {
			ls.api.LogError("Unable to create the subscription", "error", err)
			continue
		}
		go ls.refreshSubscriptionPeridically(ctx, subscriptionID)
	}
	return nil
}

func (ls *LinksService) refreshSubscriptionPeridically(ctx context.Context, subscriptionID string) error {
	err := ls.msteamsAppClient.RefreshSubscriptionPeriodically(ctx, subscriptionID)
	if err != nil {
		ls.api.LogError("error updating subscription", "error", err)
		return err
	}

	return nil
}

func (ls *LinksService) unsubscribeFromChannel(link *ChannelLink) error {
	subscriptionToRemove := ""
	for subscriptionID, subscriptionLink := range ls.subscriptionsToLinks {
		if subscriptionLink.MattermostTeam == link.MattermostTeam && subscriptionLink.MattermostChannel == link.MattermostChannel {
			subscriptionToRemove = subscriptionID
		}
	}

	if subscriptionToRemove == "" {
		return errors.New("Unable to find subscription")
	}

	err := ls.msteamsAppClient.ClearSubscription(subscriptionToRemove)
	if err != nil {
		ls.api.LogError("Unable to subscribe to channel", "error", err)
		return err
	}
	ls.subscriptionsToLinksMutex.Lock()
	delete(ls.subscriptionsToLinks, subscriptionToRemove)
	ls.subscriptionsToLinksMutex.Unlock()

	return nil
}

func (ls *LinksService) subscribeToChannel(link *ChannelLink) (string, error) {
	teamId := link.MSTeamsTeam
	channelId := link.MSTeamsChannel

	subscriptionID, err := ls.msteamsAppClient.SubscribeToChannel(teamId, channelId, ls.notificationURL, ls.webhookSecret)
	if err != nil {
		ls.api.LogError("Unable to subscribe to channel", "error", err)
		return "", err
	}
	ls.subscriptionsToLinksMutex.Lock()
	ls.subscriptionsToLinks[subscriptionID] = link
	ls.subscriptionsToLinksMutex.Unlock()
	return subscriptionID, nil
}

func (ls *LinksService) GetLinkByChannelID(channelID string) *ChannelLink {
	link, ok := ls.channelsLinked[channelID]
	if !ok {
		return nil
	}

	if !ls.checkEnabledTeamByTeamId(link.MattermostTeam) {
		return nil
	}
	return link
}

func (ls *LinksService) GetLinkBySubscriptionID(subscriptionID string) *ChannelLink {
	link, ok := ls.subscriptionsToLinks[subscriptionID]
	if !ok {
		return nil
	}

	if !ls.checkEnabledTeamByTeamId(link.MattermostTeam) {
		return nil
	}
	return link
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
	link, ok := ls.channelsLinked[channelID]
	if !ok {
		return errors.New("link doesn't exist")
	}

	delete(ls.channelsLinked, channelID)

	if err := ls.Save(); err != nil {
		ls.channelsLinked[channelID] = link
		return err
	}

	err := ls.unsubscribeFromChannel(link)
	if err != nil {
		return err
	}

	return nil
}

func (ls *LinksService) AddLink(link *ChannelLink) error {
	ls.channelsLinked[link.MattermostChannel] = link

	subscriptionID, err := ls.subscribeToChannel(link)
	if err != nil {
		return err
	}

	go ls.refreshSubscriptionPeridically(ls.stopContext, subscriptionID)

	err = ls.Save()
	if err != nil {
		delete(ls.channelsLinked, link.MattermostChannel)
		return err
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
