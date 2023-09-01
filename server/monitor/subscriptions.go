package monitor

import (
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
)

func (m *Monitor) checkChannelsSubscriptions(msteamsSubscriptionsMap map[string]*msteams.Subscription, isAllChatsSubscriptionPresent *msteams.Subscription) {
	m.api.LogDebug("Checking for channels subscriptions")
	subscriptions, err := m.store.ListChannelSubscriptionsToRefresh()
	if err != nil {
		m.api.LogError("Unable to get the channel subscriptions", "error", err)
		return
	}
	m.api.LogDebug("Refreshing channels subscriptions", "count", len(subscriptions))

	for _, subscription := range subscriptions {
		if strings.Contains(subscription.SubscriptionID, "fake-subscription-id") {
			newSubscription, err := m.client.SubscribeToChannel(subscription.TeamID, subscription.ChannelID, m.baseURL, m.webhookSecret)
			if err != nil {
				m.api.LogError("Unable to create channel subscription for fake channel subscription", "error", err)
				continue
			}

			if err := m.store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: newSubscription.ID, TeamID: subscription.TeamID, ChannelID: subscription.ChannelID, Secret: subscription.Secret, ExpiresOn: newSubscription.ExpiresOn}); err != nil {
				m.api.LogError("Unable to save channel subscription for fake channel subscription", "error", err)
			}

			if err := m.store.DeleteSubscription(subscription.SubscriptionID); err != nil {
				m.api.LogError("Unable to delete fake channel subscription", "error", err)
			}

			continue
		}

		link, _ := m.store.GetLinkByMSTeamsChannelID(subscription.TeamID, subscription.ChannelID)
		if link == nil {
			if err := m.store.DeleteSubscription(subscription.SubscriptionID); err != nil {
				m.api.LogError("Unable to delete not-needed subscription from store", "error", err)
			}

			if _, msteamsSubscriptionFound := msteamsSubscriptionsMap[subscription.SubscriptionID]; msteamsSubscriptionFound {
				if err := m.client.DeleteSubscription(subscription.SubscriptionID); err != nil {
					m.api.LogError("Unable to delete not-needed subscription from MS Teams", "error", err)
				}
			}
			continue
		}

		if _, msteamsSubscriptionFound := msteamsSubscriptionsMap[subscription.SubscriptionID]; !msteamsSubscriptionFound {
			newSubscription, err := m.client.SubscribeToChannel(subscription.TeamID, subscription.ChannelID, m.baseURL, m.webhookSecret)
			if err != nil {
				m.api.LogError("Unable to create new channel subscription", "error", err)
				continue
			}

			if err := m.store.DeleteSubscription(subscription.SubscriptionID); err != nil {
				m.api.LogWarn("Unable to delete old channel subscription", "subscriptionID", subscription.SubscriptionID, "error", err)
			}

			if err := m.store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: newSubscription.ID, TeamID: subscription.TeamID, ChannelID: subscription.ChannelID, Secret: subscription.Secret, ExpiresOn: newSubscription.ExpiresOn}); err != nil {
				m.api.LogError("Unable to store new channel subscription", "subscriptionID", newSubscription.ID, "error", err)
			}

			continue
		}

		if err := m.refreshSubscription(subscription.SubscriptionID); err != nil {
			m.api.LogDebug("Unable to refresh channel subscription properly", "error", err)
			if err := m.recreateChannelSubscription(subscription.SubscriptionID, subscription.TeamID, subscription.ChannelID, subscription.Secret); err != nil {
				m.api.LogError("Unable to recreate channel subscription properly", "error", err)
			}
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
	subscriptions, err := m.store.ListGlobalSubscriptionsToRefresh()
	if err != nil {
		m.api.LogError("Unable to get the global subscriptions", "error", err)
		return
	}

	m.api.LogDebug("Refreshing global subscriptions", "count", len(subscriptions))
	if len(subscriptions) == 0 {
		if allChatsSubscription != nil {
			if err := m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: allChatsSubscription.ID, Type: "allChats", Secret: m.webhookSecret, ExpiresOn: allChatsSubscription.ExpiresOn}); err != nil {
				m.api.LogError("Unable to store all chats subscription", "subscriptionID", allChatsSubscription.ID, "error", err)
			}
			return
		}

		m.api.LogDebug("Creating subscription for all chats")
		_ = m.createAndSaveChatSubscription(nil)
		return
	}

	mmSubscription := subscriptions[0]
	// Check if all chats subscription is not present on MS Teams
	if _, msteamsSubscriptionFound := msteamsSubscriptionsMap[mmSubscription.SubscriptionID]; !msteamsSubscriptionFound {
		// Create all chats subscription on MS Teams
		_ = m.createAndSaveChatSubscription(&mmSubscription)
		return
	}

	if err := m.refreshSubscription(mmSubscription.SubscriptionID); err != nil {
		m.api.LogDebug("Unable to refresh all chats subscription", "error", err)
		if err := m.recreateGlobalSubscription(mmSubscription.SubscriptionID, mmSubscription.Secret); err != nil {
			m.api.LogError("Unable to recreate all chats subscription", "error", err)
		}
	}
}

func (m *Monitor) createAndSaveChatSubscription(mmSubscription *storemodels.GlobalSubscription) error {
	newSubscription, err := m.client.SubscribeToChats(m.baseURL, m.webhookSecret, !m.useEvaluationAPI)
	if err != nil {
		m.api.LogError("Unable to create subscription for all chats", "error", err)
		return err
	}

	if mmSubscription != nil {
		if err := m.store.DeleteSubscription(mmSubscription.SubscriptionID); err != nil {
			m.api.LogError("Unable to delete the old all chats subscription", "error", err)
		}
	}

	if err := m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: newSubscription.ID, Type: "allChats", Secret: m.webhookSecret, ExpiresOn: newSubscription.ExpiresOn}); err != nil {
		m.api.LogError("Unable to create subscription for all chats", "error", err)
		return err
	}

	return nil
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

	if err = m.store.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogDebug("Unable to delete old channel subscription from DB", "subscriptionID", subscriptionID, "error", err.Error())
	}
	return m.store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: newSubscription.ID, TeamID: teamID, ChannelID: channelID, Secret: secret, ExpiresOn: newSubscription.ExpiresOn})
}

func (m *Monitor) recreateGlobalSubscription(subscriptionID, secret string) error {
	if err := m.client.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogDebug("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err)
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
