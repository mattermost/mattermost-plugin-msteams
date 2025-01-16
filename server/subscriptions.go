// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/pkg/errors"
)

func isExpired(expiresOn time.Time) bool {
	return expiresOn.Before(time.Now())
}

func shouldRefresh(expiresOn time.Time) bool {
	return time.Until(expiresOn) < (5 * time.Minute)
}

// seleteSubscription deletes a subscription and observing the event.
func (m *Monitor) deleteSubscription(subscriptionID string) error {
	err := m.client.DeleteSubscription(subscriptionID)
	if err != nil {
		return err
	}

	m.metrics.ObserveSubscription(metrics.SubscriptionDeleted)

	return nil
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

// checkGlobalChatsSubscription maintains the global chats subscription, creating one if it doesn't
// already exist, refreshing the expiry time as needed, or even deleting any that exists if we're
// no longer syncing direct messages.
func (m *Monitor) checkGlobalChatsSubscription(remoteSubscription *clientmodels.Subscription) {
	subscriptions, err := m.store.ListGlobalSubscriptions()
	if err != nil {
		m.api.LogWarn("Unable to get the chat subscriptions from store", "error", err.Error())
		return
	}

	// We only support one global subscription right now, and it's assumed to be the global
	// chats subscription.
	var localSubscription *storemodels.GlobalSubscription
	if len(subscriptions) > 0 {
		localSubscription = subscriptions[0]
	}

	// Delete the remote subscription if there is no local subscription, or it doesn't match the local
	// subscription. We'll continue afterwards as if there never was a remote subscription.
	if (localSubscription == nil && remoteSubscription != nil) || (localSubscription != nil && remoteSubscription != nil && remoteSubscription.ID != localSubscription.SubscriptionID) {
		m.api.LogInfo("Deleting remote global chats subscription", "subscription_id", remoteSubscription.ID)

		if err = m.deleteSubscription(remoteSubscription.ID); err != nil {
			m.api.LogError("Failed to delete remote global chats subscription", "subscription_id", remoteSubscription.ID, "error", err.Error())
			return
		}

		remoteSubscription = nil
	}

	// Try to refresh the remote subscription, if we still have one. (If we do, we know we have a matching
	// local subscription from above.)
	if remoteSubscription != nil && shouldRefresh(remoteSubscription.ExpiresOn) {
		if isExpired(remoteSubscription.ExpiresOn) {
			m.api.LogWarn("Global chats subscription discovered to be expired", "subscription_id", remoteSubscription.ID)
		}

		m.api.LogInfo("Refreshing global chats subscription", "subscription_id", remoteSubscription.ID)
		if err = m.refreshSubscription(remoteSubscription.ID); err != nil {
			m.api.LogWarn("Failed to to refresh global chats subscription", "subscription_id", remoteSubscription.ID, "error", err.Error())

			if err = m.deleteSubscription(remoteSubscription.ID); err != nil {
				m.api.LogError("Failed to delete remote global chats subscription", "subscription_id", remoteSubscription.ID, "error", err.Error())
				return
			}

			remoteSubscription = nil
		} else {
			m.api.LogInfo("Refreshed global chats subscription", "subscription_id", remoteSubscription.ID)
		}
	}

	// Delete the local subscription if there is no corresponding remote subscription. We either deleted it
	// above or it was deleted remotely, so we'll start from scratch.
	if localSubscription != nil && remoteSubscription == nil {
		m.api.LogInfo("Deleting local global chats subscription", "subscription_id", localSubscription.SubscriptionID)

		if err = m.store.DeleteSubscription(localSubscription.SubscriptionID); err != nil {
			m.api.LogError("Failed to delete local, global chats subscription", "subscription_id", localSubscription.SubscriptionID, "error", err.Error())
			return
		}

		localSubscription = nil
	}

	// At this point, we either have no subscriptions anywhere, or a matching refreshed subscription that
	// requires no more action. Just check to see if we need to create one then.
	if localSubscription == nil && remoteSubscription == nil {
		m.api.LogInfo("Creating global chats subscription")

		remoteSubscription, err = m.client.SubscribeToChats(m.baseURL, m.webhookSecret, !m.useEvaluationAPI, "")
		if err != nil {
			m.api.LogError("Failed to create global chats subscription", "error", err.Error())
			return
		}

		m.metrics.ObserveSubscription(metrics.SubscriptionConnected)

		if err := m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{
			SubscriptionID: remoteSubscription.ID,
			Type:           "allChats",
			Secret:         m.webhookSecret,
			ExpiresOn:      remoteSubscription.ExpiresOn,
		}); err != nil {
			m.api.LogError("Failed to save global chats subscription", "error", err.Error())
			return
		}

		m.api.LogInfo("Created global chats subscription", "subscription_id", remoteSubscription.ID)
	}
}

// getMSTeamsSubscriptionsMap queries MS Teams and returns a map of subscriptions indexed by
// subscription id, as well as the global chats subscription if one exists.
func (m *Monitor) getMSTeamsSubscriptionsMap() (msteamsSubscriptionsMap map[string]*clientmodels.Subscription, allChatsSubscription *clientmodels.Subscription, err error) {
	msteamsSubscriptions, err := m.client.ListSubscriptions()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to list subscriptions")
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
