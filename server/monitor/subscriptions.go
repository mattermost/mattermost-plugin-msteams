package monitor

import (
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
)

func (m *Monitor) checkChannelsSubscriptions() {
	m.api.LogDebug("Checking for channels subscriptions")
	subscriptions, err := m.store.ListChannelSubscriptionsToCheck()
	if err != nil {
		m.api.LogError("Unable to get the channel subscriptions", "error", err)
		return
	}
	m.api.LogDebug("Refreshing channels subscriptions", "count", len(subscriptions))

	for _, subscription := range subscriptions {
		if time.Until(subscription.ExpiresOn) < (15 * time.Second) {
			err := m.recreateChannelSubscription(subscription.SubscriptionID, subscription.TeamID, subscription.ChannelID, subscription.Secret)
			if err != nil {
				m.api.LogError("Unable to recreate channel subscription properly", "error", err)
			}
		} else {
			err := m.refreshSubscription(subscription.SubscriptionID)
			if err != nil {
				m.api.LogError("Unable to refresh channel subscription properly", "error", err)
			}
		}
	}
}

func (m *Monitor) checkChatsSubscriptions() {
	m.api.LogDebug("Checking for chats subscriptions")
	subscriptions, err := m.store.ListChatSubscriptionsToCheck()
	if err != nil {
		m.api.LogError("Unable to get the chat subscriptions", "error", err)
		return
	}
	m.api.LogDebug("Refreshing chats subscriptions", "count", len(subscriptions))

	for _, subscription := range subscriptions {
		if time.Until(subscription.ExpiresOn) < (15 * time.Second) {
			err := m.recreateChatSubscription(subscription.SubscriptionID, subscription.UserID, subscription.Secret)
			if err != nil {
				m.api.LogError("Unable to recreate chat subscription properly", "error", err)
			}
		} else {
			err := m.refreshSubscription(subscription.SubscriptionID)
			if err != nil {
				m.api.LogError("Unable to refresh chat subscription properly", "error", err)
			}
		}
	}
}

func (m *Monitor) checkGlobalSubscriptions() {
	m.api.LogDebug("Checking for global subscriptions")
	subscriptions, err := m.store.ListGlobalSubscriptionsToCheck()
	if err != nil {
		m.api.LogError("Unable to get the global subscriptions", "error", err)
		return
	}
	m.api.LogDebug("Refreshing global subscriptions", "count", len(subscriptions))
	for _, subscription := range subscriptions {
		if time.Until(subscription.ExpiresOn) < (15 * time.Second) {
			err := m.recreateGlobalSubscription(subscription.SubscriptionID, subscription.Type, subscription.Secret)
			if err != nil {
				m.api.LogError("Unable to recreate global subscription properly", "error", err)
			}
		} else {
			err := m.refreshSubscription(subscription.SubscriptionID)
			if err != nil {
				m.api.LogError("Unable to refresh global subscription properly", "error", err)
			}
		}
	}
}

func (m *Monitor) recreateChatSubscription(subscriptionID, userID, secret string) error {
	err := m.client.DeleteSubscription(subscriptionID)
	if err != nil {
		return err
	}
	newSubscription, err := m.client.SubscribeToUserChats(userID, m.baseURL, m.webhookSecret, !m.evaluationAPI)
	if err != nil {
		return err
	}
	if err := m.store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: newSubscription.ID, UserID: userID, Secret: secret, ExpiresOn: newSubscription.ExpiresOn}); err != nil {
		return err
	}
	return nil
}

func (m *Monitor) recreateChannelSubscription(subscriptionID, teamID, secret, channelID string) error {
	err := m.client.DeleteSubscription(subscriptionID)
	if err != nil {
		return err
	}

	newSubscription, err := m.client.SubscribeToChannel(teamID, channelID, m.baseURL, m.webhookSecret)
	if err != nil {
		return err
	}

	if err := m.store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: newSubscription.ID, TeamID: teamID, ChannelID: channelID, Secret: secret, ExpiresOn: newSubscription.ExpiresOn}); err != nil {
		return err
	}
	return nil
}

func (m *Monitor) recreateGlobalSubscription(subscriptionID, subscriptionType, secret string) error {
	err := m.client.DeleteSubscription(subscriptionID)
	if err != nil {
		return err
	}

	var newSubscription *msteams.Subscription
	if subscriptionType == "allChannels" {
		newSubscription, err = m.client.SubscribeToChannels(m.baseURL, m.webhookSecret, !m.evaluationAPI)
		if err != nil {
			return err
		}
	} else {
		newSubscription, err = m.client.SubscribeToChats(m.baseURL, m.webhookSecret, !m.evaluationAPI)
		if err != nil {
			return err
		}
	}

	if err := m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: newSubscription.ID, Type: subscriptionType, Secret: secret, ExpiresOn: newSubscription.ExpiresOn}); err != nil {
		return err
	}
	return nil
}

func (m *Monitor) refreshSubscription(subscriptionID string) error {
	newSubscriptionTime, err := m.client.RefreshSubscription(subscriptionID)
	if err != nil {
		return err
	}
	if err := m.store.UpdateSubscriptionExpiresOn(subscriptionID, *newSubscriptionTime); err != nil {
		return err
	}
	return nil
}
