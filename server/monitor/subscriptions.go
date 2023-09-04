package monitor

import (
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
)

func (m *Monitor) checkChannelsSubscriptions(msteamsSubscriptionsMap map[string]*msteams.Subscription) {
	m.api.LogDebug("Checking for channels subscriptions")
	subscriptions, err := m.store.ListChannelSubscriptionsToRefresh()
	if err != nil {
		m.api.LogError("Unable to get the channel subscriptions", "error", err.Error())
		return
	}
	m.api.LogDebug("Refreshing channels subscriptions", "count", len(subscriptions))

	for _, subscription := range subscriptions {
		if strings.Contains(subscription.SubscriptionID, "fake-subscription-id") {
			m.recreateChannelSubscription(subscription.SubscriptionID, subscription.TeamID, subscription.ChannelID, m.webhookSecret, false)
			continue
		}

		link, _ := m.store.GetLinkByMSTeamsChannelID(subscription.TeamID, subscription.ChannelID)
		if link == nil {
			if err = m.store.DeleteSubscription(subscription.SubscriptionID); err != nil {
				m.api.LogError("Unable to delete not-needed subscription from store", "error", err.Error())
			}

			if _, msteamsSubscriptionFound := msteamsSubscriptionsMap[subscription.SubscriptionID]; msteamsSubscriptionFound {
				if err = m.client.DeleteSubscription(subscription.SubscriptionID); err != nil {
					m.api.LogError("Unable to delete not-needed subscription from MS Teams", "error", err.Error())
				}
			}
			continue
		}

		if _, msteamsSubscriptionFound := msteamsSubscriptionsMap[subscription.SubscriptionID]; !msteamsSubscriptionFound {
			m.recreateChannelSubscription(subscription.SubscriptionID, subscription.TeamID, subscription.ChannelID, m.webhookSecret, false)
			continue
		}

		if err := m.refreshSubscription(subscription.SubscriptionID); err != nil {
			m.api.LogDebug("Unable to refresh channel subscription properly", "error", err.Error())
			m.recreateChannelSubscription(subscription.SubscriptionID, subscription.TeamID, subscription.ChannelID, subscription.Secret, true)
		}
	}
}

// Commenting the below function as we are not creating any user type subscriptions
// func (m *Monitor) checkChatsSubscriptions() {
// 	m.api.LogDebug("Checking for chats subscriptions")
// 	subscriptions, err := m.store.ListChatSubscriptionsToCheck()
// 	if err != nil {
// 		m.api.LogError("Unable to get the chat subscriptions", "error", err)
// 		return
// 	}
// 	m.api.LogDebug("Refreshing chats subscriptions", "count", len(subscriptions))

// 	for _, subscription := range subscriptions {
// 		if time.Until(subscription.ExpiresOn) < (15 * time.Second) {
// 			if err := m.recreateChatSubscription(subscription.SubscriptionID, subscription.UserID, subscription.Secret); err != nil {
// 				m.api.LogError("Unable to recreate chat subscription properly", "error", err)
// 			}
// 		} else {
// 			if err := m.refreshSubscription(subscription.SubscriptionID); err != nil {
// 				m.api.LogDebug("Unable to refresh chat subscription properly", "error", err)
// 				if err := m.recreateChatSubscription(subscription.SubscriptionID, subscription.UserID, subscription.Secret); err != nil {
// 					m.api.LogError("Unable to recreate chat subscription properly", "error", err)
// 				}
// 			}
// 		}
// 	}
// }

func (m *Monitor) checkGlobalSubscriptions(msteamsSubscriptionsMap map[string]*msteams.Subscription, allChatsSubscription *msteams.Subscription) {
	m.api.LogDebug("Checking for global subscriptions")
	subscriptions, err := m.store.ListGlobalSubscriptions()
	if err != nil {
		m.api.LogError("Unable to get the chat subscriptions from store", "error", err.Error())
		return
	}

	m.api.LogDebug("Refreshing global subscriptions", "count", len(subscriptions))

	if len(subscriptions) == 0 {
		if allChatsSubscription == nil {
			m.CreateAndSaveChatSubscription(nil)
		} else {
			if err := m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: allChatsSubscription.ID, Type: "allChats", ExpiresOn: allChatsSubscription.ExpiresOn, Secret: m.webhookSecret}); err != nil {
				m.api.LogError("Unable to store all chats subscription in store", "subscriptionID", allChatsSubscription.ID, "error", err.Error())
			}
		}

		return
	}

	mmSubscription := subscriptions[0]
	// Check if all chats subscription is not present on MS Teams
	if _, msteamsSubscriptionFound := msteamsSubscriptionsMap[mmSubscription.SubscriptionID]; !msteamsSubscriptionFound {
		// Create all chats subscription on MS Teams
		m.CreateAndSaveChatSubscription(mmSubscription)
		return
	}

	if time.Until(mmSubscription.ExpiresOn) < (5 * time.Minute) {
		if err := m.refreshSubscription(mmSubscription.SubscriptionID); err != nil {
			m.api.LogDebug("Unable to refresh all chats subscription", "error", err.Error())
			if err := m.recreateGlobalSubscription(mmSubscription.SubscriptionID, mmSubscription.Secret); err != nil {
				m.api.LogError("Unable to recreate all chats subscription", "error", err.Error())
			}
		}
	}
}

func (m *Monitor) CreateAndSaveChatSubscription(mmSubscription *storemodels.GlobalSubscription) {
	newSubscription, err := m.client.SubscribeToChats(m.baseURL, m.webhookSecret, !m.useEvaluationAPI)
	if err != nil {
		m.api.LogError("Unable to create subscription for all chats", "error", err.Error())
		return
	}

	if mmSubscription != nil {
		if err := m.store.DeleteSubscription(mmSubscription.SubscriptionID); err != nil {
			m.api.LogError("Unable to delete the old all chats subscription", "error", err.Error())
		}
	}

	if err := m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: newSubscription.ID, Type: "allChats", Secret: m.webhookSecret, ExpiresOn: newSubscription.ExpiresOn}); err != nil {
		m.api.LogError("Unable to create subscription for all chats", "error", err.Error())
		return
	}
}

func (m *Monitor) recreateChatSubscription(subscriptionID, userID, secret string) error {
	if err := m.client.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogDebug("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err.Error())
	}

	newSubscription, err := m.client.SubscribeToUserChats(userID, m.baseURL, m.webhookSecret, !m.useEvaluationAPI)
	if err != nil {
		return err
	}

	return m.store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: newSubscription.ID, UserID: userID, Secret: secret, ExpiresOn: newSubscription.ExpiresOn})
}

func (m *Monitor) recreateChannelSubscription(subscriptionID, teamID, channelID, secret string, deleteFromClient bool) {
	if deleteFromClient && subscriptionID != "" {
		if err := m.client.DeleteSubscription(subscriptionID); err != nil {
			m.api.LogDebug("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err.Error())
		}
	}

	newSubscription, err := m.client.SubscribeToChannel(teamID, channelID, m.baseURL, m.webhookSecret)
	if err != nil {
		m.api.LogError("Unable to create new subscription for the channel", "channelID", channelID, "error", err.Error())
		return
	}

	if err = m.store.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogDebug("Unable to delete old channel subscription from DB", "subscriptionID", subscriptionID, "error", err.Error())
	}

	if err := m.store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: newSubscription.ID, TeamID: teamID, ChannelID: channelID, Secret: secret, ExpiresOn: newSubscription.ExpiresOn}); err != nil {
		m.api.LogError("Unable to store new subscription in DB", "subscriptionID", newSubscription.ID, "error", err.Error())
		return
	}
}

func (m *Monitor) recreateGlobalSubscription(subscriptionID, secret string) error {
	if err := m.client.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogDebug("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err.Error())
	}

	newSubscription, err := m.client.SubscribeToChats(m.baseURL, secret, !m.useEvaluationAPI)
	if err != nil {
		return err
	}

	if err = m.store.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogDebug("Unable to delete old global subscription from DB", "subscriptionID", subscriptionID, "error", err.Error())
	}
	return m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: newSubscription.ID, Type: "allChats", Secret: secret, ExpiresOn: newSubscription.ExpiresOn})
}

func (m *Monitor) refreshSubscription(subscriptionID string) error {
	newSubscriptionTime, err := m.client.RefreshSubscription(subscriptionID)
	if err != nil {
		return err
	}
	return m.store.UpdateSubscriptionExpiresOn(subscriptionID, *newSubscriptionTime)
}

func (m *Monitor) GetMSTeamsSubscriptionsMap() (msteamsSubscriptionsMap map[string]*msteams.Subscription, allChatsSubscription *msteams.Subscription, err error) {
	msteamsSubscriptions, err := m.client.ListSubscriptions()
	if err != nil {
		m.api.LogError("Unable to list MS Teams subscriptions", "error", err.Error())
		return nil, nil, err
	}

	msteamsSubscriptionsMap = make(map[string]*msteams.Subscription)
	for _, msteamsSubscription := range msteamsSubscriptions {
		msteamsSubscriptionsMap[msteamsSubscription.ID] = msteamsSubscription
		if strings.Contains(msteamsSubscription.Resource, "chats/getAllMessages") {
			allChatsSubscription = msteamsSubscription
		}
	}

	return
}
