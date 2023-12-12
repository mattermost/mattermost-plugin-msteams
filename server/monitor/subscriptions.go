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

	m.api.LogDebug("Checking for channels subscriptions")
	links, err := m.store.ListChannelLinks()
	if err != nil {
		m.api.LogError("Unable to list channel links from DB", "error", err.Error())
		return
	}

	subscriptions, err := m.store.ListChannelSubscriptions()
	if err != nil {
		m.api.LogError("Unable to get the channel subscriptions", "error", err.Error())
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
				if _, msteamsSubscriptionFound := msteamsSubscriptionsMap[mmSubscription.SubscriptionID]; !msteamsSubscriptionFound {
					// Create channel subscription for the linked channel
					m.recreateChannelSubscription(mmSubscription.SubscriptionID, mmSubscription.TeamID, mmSubscription.ChannelID, m.webhookSecret, false)
					<-ws
					return
				} else if mmSubscription.Certificate != m.certificate {
					m.recreateChannelSubscription(mmSubscription.SubscriptionID, mmSubscription.TeamID, mmSubscription.ChannelID, mmSubscription.Secret, true)
				} else if time.Until(mmSubscription.ExpiresOn) < (5 * time.Minute) {
					if err := m.refreshSubscription(mmSubscription.SubscriptionID); err != nil {
						m.api.LogDebug("Unable to refresh channel subscription", "error", err.Error())
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

func (m *Monitor) checkGlobalSubscriptions(msteamsSubscriptionsMap map[string]*clientmodels.Subscription, allChatsSubscription *clientmodels.Subscription) {
	m.api.LogDebug("Checking for global subscriptions")
	subscriptions, err := m.store.ListGlobalSubscriptions()
	if err != nil {
		m.api.LogError("Unable to get the chat subscriptions from store", "error", err.Error())
		return
	}

	if len(subscriptions) == 0 {
		if allChatsSubscription == nil {
			m.CreateAndSaveChatSubscription(nil)
		} else {
			if err := m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: allChatsSubscription.ID, Type: "allChats", ExpiresOn: allChatsSubscription.ExpiresOn, Secret: m.webhookSecret, Certificate: m.certificate}); err != nil {
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

	if mmSubscription.Certificate != m.certificate {
		if err := m.recreateGlobalSubscription(mmSubscription.SubscriptionID, mmSubscription.Secret); err != nil {
			m.api.LogError("Unable to recreate all chats subscription", "error", err.Error())
		}
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
	newSubscription, err := m.client.SubscribeToChats(m.baseURL, m.webhookSecret, !m.useEvaluationAPI, m.certificate)
	if err != nil {
		m.api.LogError("Unable to create subscription for all chats", "error", err.Error())
		return
	}

	m.metrics.ObserveSubscription(metrics.SubscriptionConnected)

	if mmSubscription != nil {
		if err := m.store.DeleteSubscription(mmSubscription.SubscriptionID); err != nil {
			m.api.LogError("Unable to delete the old all chats subscription", "error", err.Error())
		}
	}

	if err := m.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: newSubscription.ID, Type: "allChats", Secret: m.webhookSecret, ExpiresOn: newSubscription.ExpiresOn, Certificate: m.certificate}); err != nil {
		m.api.LogError("Unable to create subscription for all chats", "error", err.Error())
		return
	}
}

func (m *Monitor) recreateChatSubscription(subscriptionID, userID, secret string) error {
	if err := m.client.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogDebug("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err.Error())
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
			m.api.LogDebug("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err.Error())
		}
	}

	newSubscription, err := m.client.SubscribeToChannel(teamID, channelID, m.baseURL, m.webhookSecret, m.certificate)
	if err != nil {
		m.api.LogError("Unable to create new subscription for the channel", "channelID", channelID, "error", err.Error())
		return
	}

	m.metrics.ObserveSubscription(metrics.SubscriptionReconnected)

	if subscriptionID != "" {
		if err = m.store.DeleteSubscription(subscriptionID); err != nil {
			m.api.LogDebug("Unable to delete old channel subscription from DB", "subscriptionID", subscriptionID, "error", err.Error())
		}
	}

	tx, err := m.store.BeginTx()
	if err != nil {
		m.api.LogWarn("Unable to begin database transaction", "error", err.Error())
		return
	}

	var txErr error
	defer func() {
		if txErr != nil {
			if err := m.store.RollbackTx(tx); err != nil {
				m.api.LogWarn("Unable to rollback database transaction", "error", err.Error())
			}
			return
		}

		if err := m.store.CommitTx(tx); err != nil {
			m.api.LogWarn("Unable to commit database transaction", "error", err.Error())
		}
	}()

	if txErr = m.store.SaveChannelSubscription(tx, storemodels.ChannelSubscription{SubscriptionID: newSubscription.ID, TeamID: teamID, ChannelID: channelID, Secret: secret, ExpiresOn: newSubscription.ExpiresOn, Certificate: m.certificate}); txErr != nil {
		m.api.LogError("Unable to store new subscription in DB", "subscriptionID", newSubscription.ID, "error", txErr.Error())
		return
	}
}

func (m *Monitor) recreateGlobalSubscription(subscriptionID, secret string) error {
	if err := m.client.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogDebug("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err.Error())
	}

	newSubscription, err := m.client.SubscribeToChats(m.baseURL, secret, !m.useEvaluationAPI, m.certificate)
	if err != nil {
		return err
	}

	m.metrics.ObserveSubscription(metrics.SubscriptionReconnected)

	if err = m.store.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogDebug("Unable to delete old global subscription from DB", "subscriptionID", subscriptionID, "error", err.Error())
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
