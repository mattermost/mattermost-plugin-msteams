package monitor

import (
	"errors"
	"time"

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
		link, _ := m.store.GetLinkByMSTeamsChannelID(subscription.TeamID, subscription.ChannelID)
		if link == nil {
			if err := m.store.DeleteSubscription(subscription.SubscriptionID); err != nil {
				m.api.LogError("Unable to delete not-needed subscription", "error", err)
			}
			// Ignoring the error because can be the case that the subscription is no longer exists, in that case, it doesn't matter.
			_ = m.client.DeleteSubscription(subscription.SubscriptionID)
			continue
		}

		if time.Until(subscription.ExpiresOn) < (15 * time.Second) {
			if err := m.recreateChannelSubscription(subscription.SubscriptionID, subscription.TeamID, subscription.ChannelID, subscription.Secret); err != nil {
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
			if err := m.refreshSubscription(subscription.SubscriptionID); err != nil {
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
			if err := m.recreateGlobalSubscription(subscription.SubscriptionID, subscription.Type, subscription.Secret); err != nil {
				m.api.LogError("Unable to recreate global subscription properly", "error", err)
			}
		} else {
			if err := m.refreshSubscription(subscription.SubscriptionID); err != nil {
				m.api.LogError("Unable to refresh global subscription properly", "error", err)
			}
		}
	}
}

func (m *Monitor) recreateChatSubscription(subscriptionID, userID, secret string) error {
	if err := m.client.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogDebug("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err)
	}

	newSubscription, err := m.client.SubscribeToUserChats(userID, m.baseURL, m.webhookSecret, !m.useEvaluationAPI)
	if err != nil {
		return err
	}

	return m.store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: newSubscription.ID, UserID: userID, Secret: secret, ExpiresOn: newSubscription.ExpiresOn})
}

func (m *Monitor) recreateChannelSubscription(subscriptionID, teamID, channelID, secret string) error {
	if err := m.client.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogDebug("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err)
	}

	newSubscription, err := m.client.SubscribeToChannel(teamID, channelID, m.baseURL, m.webhookSecret)
	if err != nil {
		return err
	}

	return m.store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: newSubscription.ID, TeamID: teamID, ChannelID: channelID, Secret: secret, ExpiresOn: newSubscription.ExpiresOn})
}

func (m *Monitor) recreateGlobalSubscription(subscriptionID, subscriptionType, secret string) error {
	if err := m.client.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogDebug("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err)
	}

	if subscriptionType != "allChats" {
		return errors.New("invalid subscription type")
	}

	newSubscription, err := m.client.SubscribeToChats(m.baseURL, secret, !m.useEvaluationAPI)
	if err != nil {
		return err
	}

	return m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: newSubscription.ID, Type: subscriptionType, Secret: secret, ExpiresOn: newSubscription.ExpiresOn})
}

func (m *Monitor) refreshSubscription(subscriptionID string) error {
	newSubscriptionTime, err := m.client.RefreshSubscription(subscriptionID)
	if err != nil {
		return err
	}
	return m.store.UpdateSubscriptionExpiresOn(subscriptionID, *newSubscriptionTime)
}
