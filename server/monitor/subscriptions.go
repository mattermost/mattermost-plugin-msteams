package monitor

import "time"

func (m *Monitor) checkChannelsSubscriptions() {
	subscriptions := m.store.ListChannelSubscriptionsToCheck()
	for _, subscription := range subscriptions {
		if time.Until(subscription.ExpiresOn) < (15 * time.Second) {
			err := m.recreateChannelSubscription(subscription.SubscriptionID, subscription.TeamID, subscription.ChannelID)
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
	subscriptions := m.store.ListChatSubscriptionsToCheck()
	for _, subscription := range subscriptions {
		if time.Until(subscription.ExpiresOn) < (15 * time.Second) {
			err := m.recreateChatSubscription(subscription.SubscriptionID, subscription.UserID)
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

func (m *Monitor) recreateChatSubscription(subscriptionID, userID string) error {
	err := m.client.DeleteSubscription(subscriptionID)
	if err != nil {
		return err
	}
	newSubscription, err := m.client.SubscribeToUserChats(userID, m.baseURL, m.webhookSecret, !m.evaluationAPI)
	if err != nil {
		return err
	}
	if err := m.store.SaveSubscription(newSubscription.ID, userID, "", "", newSubscription.ExpiresOn); err != nil {
		return err
	}
	return nil
}

func (m *Monitor) recreateChannelSubscription(subscriptionID, teamID, channelID string) error {
	err := m.client.DeleteSubscription(subscriptionID)
	if err != nil {
		return err
	}

	newSubscription, err := m.client.SubscribeToChannel(teamID, channelID, m.baseURL, m.webhookSecret)
	if err != nil {
		return err
	}

	if err := m.store.SaveSubscription(newSubscription.ID, "", teamID, channelID, newSubscription.ExpiresOn); err != nil {
		return err
	}
	return nil
}

func (m *Monitor) refreshSubscription(subscriptionID string) error {
	return m.client.RefreshSubscription(subscriptionID)
}
