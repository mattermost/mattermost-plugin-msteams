package main

import (
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
)

func isExpired(expiresOn time.Time) bool {
	return expiresOn.Before(time.Now())
}

func shouldRefresh(expiresOn time.Time) bool {
	return time.Until(expiresOn) < (5 * time.Minute)
}

// checkGlobalChatsSubscription maintains the global chats subscription, creating one if it doesn't
// already exist, refreshing the expiry time as needed, or even deleting any that exists if we're
// no longer syncing direct messages.
func (m *Monitor) checkGlobalChatsSubscription(msteamsSubscriptionsMap map[string]*clientmodels.Subscription, allChatsSubscription *clientmodels.Subscription) {
	subscriptions, err := m.store.ListGlobalSubscriptions()
	if err != nil {
		m.api.LogWarn("Unable to get the chat subscriptions from store", "error", err.Error())
		return
	}

	// Create or save a global subscription if we have none.
	if len(subscriptions) == 0 {
		if allChatsSubscription == nil {
			m.createAndSaveChatSubscription(nil)
		} else {
			if err := m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: allChatsSubscription.ID, Type: "allChats", ExpiresOn: allChatsSubscription.ExpiresOn, Secret: m.webhookSecret}); err != nil {
				m.api.LogWarn("Unable to store all chats subscription in store", "subscription_id", allChatsSubscription.ID, "error", err.Error())
			}
		}

		return
	}

	// We only support one global subscription right now, and it's assumed to be the global
	// chats subscription.
	mmSubscription := subscriptions[0]

	// Check if all chats subscription is not present on MS Teams
	if _, msteamsSubscriptionFound := msteamsSubscriptionsMap[mmSubscription.SubscriptionID]; !msteamsSubscriptionFound {
		m.api.LogInfo("Creating global chats subscription")

		// Create all chats subscription on MS Teams
		m.createAndSaveChatSubscription(mmSubscription)
		return
	}

	if shouldRefresh(mmSubscription.ExpiresOn) {
		if isExpired(mmSubscription.ExpiresOn) {
			// In the future, this won't need to be an error if we can resync, but for
			// now notify a human.
			m.api.LogError("Global chats subscription expired")
		}

		m.api.LogInfo("Renewing global chats subscription")
		if err := m.refreshSubscription(mmSubscription.SubscriptionID); err != nil {
			m.api.LogWarn("Failed to to refresh global chats subscription, recreating", "error", err.Error())
			if err := m.recreateGlobalSubscription(mmSubscription.SubscriptionID, mmSubscription.Secret); err != nil {
				m.api.LogError("Unable to recreate all chats subscription", "error", err.Error())
			}
		}
	}
}

// createAndSaveChatSubscription creates a chats/getAllMessages subscription, observing the event
// as a metric, recording the new subscription in the database, and deleting the old global chats
// subscription if given.
func (m *Monitor) createAndSaveChatSubscription(mmSubscription *storemodels.GlobalSubscription) {
	newSubscription, err := m.client.SubscribeToChats(m.baseURL, m.webhookSecret, !m.useEvaluationAPI, "")
	if err != nil {
		m.api.LogError("Unable to create subscription for all chats", "error", err.Error())
		return
	}

	m.metrics.ObserveSubscription(metrics.SubscriptionConnected)

	if mmSubscription != nil {
		if err := m.store.DeleteSubscription(mmSubscription.SubscriptionID); err != nil {
			m.api.LogWarn("Unable to delete the old all chats subscription", "error", err.Error())
		}
	}

	if err := m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: newSubscription.ID, Type: "allChats", Secret: m.webhookSecret, ExpiresOn: newSubscription.ExpiresOn}); err != nil {
		m.api.LogError("Unable to create subscription for all chats", "error", err.Error())
		return
	}
}

// recreateGlobalSubscription deletes the existing chats/getAllMessages subscription (if it exists)
// and recreates it, observing the event as a metric and recording the new subscription in the
// database.
func (m *Monitor) recreateGlobalSubscription(subscriptionID, secret string) error {
	if err := m.client.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogWarn("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err.Error())
	}

	newSubscription, err := m.client.SubscribeToChats(m.baseURL, secret, !m.useEvaluationAPI, "")
	if err != nil {
		return err
	}

	m.metrics.ObserveSubscription(metrics.SubscriptionReconnected)

	if err = m.store.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogWarn("Unable to delete old global subscription from DB", "subscription_id", subscriptionID, "error", err.Error())
	}
	return m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: newSubscription.ID, Type: "allChats", Secret: secret, ExpiresOn: newSubscription.ExpiresOn})
}

// refreshSubscription renews a subscription by extending its expiry time, observing the event as
// a metric and recording the new expiry timestamp in the database.
func (m *Monitor) refreshSubscription(subscriptionID string) error {
	newSubscriptionTime, err := m.client.RefreshSubscription(subscriptionID)
	if err != nil {
		return err
	}

	m.metrics.ObserveSubscription(metrics.SubscriptionRefreshed)

	return m.store.UpdateSubscriptionExpiresOn(subscriptionID, *newSubscriptionTime)
}

// getMSTeamsSubscriptionsMap queries MS Teams and returns a map of subscriptions indexed by
// subscription id, as well as the global chats subscription if one exists.
func (m *Monitor) getMSTeamsSubscriptionsMap() (msteamsSubscriptionsMap map[string]*clientmodels.Subscription, allChatsSubscription *clientmodels.Subscription, err error) {
	msteamsSubscriptions, err := m.client.ListSubscriptions()
	if err != nil {
		m.api.LogError("Unable to list MS Teams subscriptions", "error", err.Error())
		return nil, nil, err
	}

	msteamsSubscriptionsMap = make(map[string]*clientmodels.Subscription)
	for _, msteamsSubscription := range msteamsSubscriptions {
		if strings.HasPrefix(msteamsSubscription.NotificationURL, m.baseURL) {
			msteamsSubscriptionsMap[msteamsSubscription.ID] = msteamsSubscription
			if strings.Contains(msteamsSubscription.Resource, "chats/getAllMessages") {
				allChatsSubscription = msteamsSubscription
			}
		}
	}

	return
}
