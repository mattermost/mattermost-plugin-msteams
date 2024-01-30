package monitor

import (
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
)

func (m *Monitor) checkChannelsSubscriptions(msteamsSubscriptionsMap map[string]*clientmodels.Subscription) {
	defer func() {
		if r := recover(); r != nil {
			m.metrics.ObserveGoroutineFailure()
			m.api.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	links, err := m.store.ListChannelLinks()
	if err != nil {
		m.api.LogWarn("Unable to list channel links from DB", "error", err.Error())
		return
	}

	subscriptions, err := m.store.ListChannelSubscriptions()
	if err != nil {
		m.api.LogWarn("Unable to get the channel subscriptions", "error", err.Error())
		return
	}

	channelSubscriptionsMap := make(map[string]*storemodels.ChannelSubscription)
	for _, subscription := range subscriptions {
		channelSubscriptionsMap[subscription.TeamID+subscription.ChannelID] = subscription
	}

	wg := sync.WaitGroup{}
	ws := make(chan struct{}, 20)

	for _, link := range links {
		ws <- struct{}{}
		wg.Add(1)

		go func(link storemodels.ChannelLink) {
			defer func() {
				if r := recover(); r != nil {
					m.metrics.ObserveGoroutineFailure()
					m.api.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
				}
			}()

			defer wg.Done()
			mmSubscription, mmSubscriptionFound := channelSubscriptionsMap[link.MSTeamsTeam+link.MSTeamsChannel]
			// Check if channel subscription is present for a link on Mattermost
			if mmSubscriptionFound {
				// Check if channel subscription is not present on MS Teams
				_, msteamsSubscriptionFound := msteamsSubscriptionsMap[mmSubscription.SubscriptionID]
				switch {
				case !msteamsSubscriptionFound:
					// Create channel subscription for the linked channel
					m.recreateChannelSubscription(mmSubscription.SubscriptionID, mmSubscription.TeamID, mmSubscription.ChannelID, m.webhookSecret, false)
					<-ws
					return

				case mmSubscription.Certificate != m.certificate:
					m.recreateChannelSubscription(mmSubscription.SubscriptionID, mmSubscription.TeamID, mmSubscription.ChannelID, mmSubscription.Secret, true)

				case time.Until(mmSubscription.ExpiresOn) < (5 * time.Minute):
					if err := m.refreshSubscription(mmSubscription.SubscriptionID); err != nil {
						m.api.LogError("Unable to refresh channel subscription", "error", err.Error())
						m.recreateChannelSubscription(mmSubscription.SubscriptionID, mmSubscription.TeamID, mmSubscription.ChannelID, mmSubscription.Secret, true)
					}
				}
			} else {
				// Create channel subscription for the linked channel
				m.recreateChannelSubscription("", link.MSTeamsTeam, link.MSTeamsChannel, m.webhookSecret, false)
				<-ws
				return
			}
			<-ws
		}(link)
	}
	wg.Wait()
}

// Commenting the below function as we are not creating any user type subscriptions
// func (m *Monitor) checkChatsSubscriptions() {
// 	subscriptions, err := m.store.ListChatSubscriptionsToCheck()
// 	if err != nil {
// 		m.api.LogWarn("Unable to get the chat subscriptions", "error", err)
// 		return
// 	}
// 	m.api.LogInfo("Refreshing chats subscriptions", "count", len(subscriptions))

// 	for _, subscription := range subscriptions {
// 		if time.Until(subscription.ExpiresOn) < (15 * time.Second) {
// 			if err := m.recreateChatSubscription(subscription.SubscriptionID, subscription.UserID, subscription.Secret); err != nil {
// 				m.api.LogError("Unable to recreate chat subscription properly", "error", err)
// 			}
// 		} else {
// 			if err := m.refreshSubscription(subscription.SubscriptionID); err != nil {
// 				m.api.LogError("Unable to refresh chat subscription properly", "error", err)
// 				if err := m.recreateChatSubscription(subscription.SubscriptionID, subscription.UserID, subscription.Secret); err != nil {
// 					m.api.LogError("Unable to recreate chat subscription properly", "error", err)
// 				}
// 			}
// 		}
// 	}
// }

func (m *Monitor) checkGlobalSubscriptions(msteamsSubscriptionsMap map[string]*clientmodels.Subscription, allChatsSubscription *clientmodels.Subscription) {
	subscriptions, err := m.store.ListGlobalSubscriptions()
	if err != nil {
		m.api.LogWarn("Unable to get the chat subscriptions from store", "error", err.Error())
		return
	}

	if len(subscriptions) == 0 {
		if allChatsSubscription == nil {
			m.CreateAndSaveChatSubscription(nil)
		} else {
			if err := m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: allChatsSubscription.ID, Type: "allChats", ExpiresOn: allChatsSubscription.ExpiresOn, Secret: m.webhookSecret, Certificate: m.certificate}); err != nil {
				m.api.LogWarn("Unable to store all chats subscription in store", "subscription_id", allChatsSubscription.ID, "error", err.Error())
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

	if mmSubscription.Certificate != m.certificate {
		if err := m.recreateGlobalSubscription(mmSubscription.SubscriptionID, mmSubscription.Secret); err != nil {
			m.api.LogError("Unable to recreate all chats subscription", "error", err.Error())
		}
	}

	if time.Until(mmSubscription.ExpiresOn) < (5 * time.Minute) {
		if err := m.refreshSubscription(mmSubscription.SubscriptionID); err != nil {
			m.api.LogError("Unable to refresh all chats subscription", "error", err.Error())
			if err := m.recreateGlobalSubscription(mmSubscription.SubscriptionID, mmSubscription.Secret); err != nil {
				m.api.LogError("Unable to recreate all chats subscription", "error", err.Error())
			}
		}
	}
}

func (m *Monitor) CreateAndSaveChatSubscription(mmSubscription *storemodels.GlobalSubscription) {
	newSubscription, err := m.client.SubscribeToChats(m.baseURL, m.webhookSecret, !m.useEvaluationAPI, m.certificate)
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

	if err := m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: newSubscription.ID, Type: "allChats", Secret: m.webhookSecret, ExpiresOn: newSubscription.ExpiresOn, Certificate: m.certificate}); err != nil {
		m.api.LogError("Unable to create subscription for all chats", "error", err.Error())
		return
	}
}

func (m *Monitor) recreateChatSubscription(subscriptionID, userID, secret string) error {
	if err := m.client.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogWarn("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err.Error())
	}

	newSubscription, err := m.client.SubscribeToUserChats(userID, m.baseURL, m.webhookSecret, !m.useEvaluationAPI, m.certificate)
	if err != nil {
		return err
	}

	return m.store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: newSubscription.ID, UserID: userID, Secret: secret, ExpiresOn: newSubscription.ExpiresOn, Certificate: m.certificate})
}

func (m *Monitor) recreateChannelSubscription(subscriptionID, teamID, channelID, secret string, deleteFromClient bool) {
	if deleteFromClient && subscriptionID != "" {
		if err := m.client.DeleteSubscription(subscriptionID); err != nil {
			m.api.LogWarn("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err.Error())
		}
	}

	newSubscription, err := m.client.SubscribeToChannel(teamID, channelID, m.baseURL, m.webhookSecret, m.certificate)
	if err != nil {
		m.api.LogError("Unable to create new subscription for the channel", "channel_id", channelID, "error", err.Error())
		return
	}

	m.metrics.ObserveSubscription(metrics.SubscriptionReconnected)

	if subscriptionID != "" {
		if err = m.store.DeleteSubscription(subscriptionID); err != nil {
			m.api.LogWarn("Unable to delete old channel subscription from DB", "subscription_id", subscriptionID, "error", err.Error())
		}
	}

	if err = m.store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: newSubscription.ID, TeamID: teamID, ChannelID: channelID, Secret: secret, ExpiresOn: newSubscription.ExpiresOn, Certificate: m.certificate}); err != nil {
		m.api.LogError("Unable to store new subscription in DB", "subscription_id", newSubscription.ID, "error", err.Error())
		return
	}
}

func (m *Monitor) recreateGlobalSubscription(subscriptionID, secret string) error {
	if err := m.client.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogWarn("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err.Error())
	}

	newSubscription, err := m.client.SubscribeToChats(m.baseURL, secret, !m.useEvaluationAPI, m.certificate)
	if err != nil {
		return err
	}

	m.metrics.ObserveSubscription(metrics.SubscriptionReconnected)

	if err = m.store.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogWarn("Unable to delete old global subscription from DB", "subscription_id", subscriptionID, "error", err.Error())
	}
	return m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: newSubscription.ID, Type: "allChats", Secret: secret, ExpiresOn: newSubscription.ExpiresOn, Certificate: m.certificate})
}

func (m *Monitor) refreshSubscription(subscriptionID string) error {
	newSubscriptionTime, err := m.client.RefreshSubscription(subscriptionID)
	if err != nil {
		return err
	}

	m.metrics.ObserveSubscription(metrics.SubscriptionRefreshed)

	return m.store.UpdateSubscriptionExpiresOn(subscriptionID, *newSubscriptionTime)
}

func (m *Monitor) GetMSTeamsSubscriptionsMap() (msteamsSubscriptionsMap map[string]*clientmodels.Subscription, allChatsSubscription *clientmodels.Subscription, err error) {
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
