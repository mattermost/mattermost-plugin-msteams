package main

import (
	"runtime/debug"
	"strings"
	"sync"
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

// checkChannelsSubscriptions maintains the per-channel subscriptions, creating one if it doesn't
// already exist or refreshing the expiry time as needed.
//
// Subscriptions are refreshed in parallel, processing at most 20 concurrently.
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
					m.api.LogInfo("Creating subscription for channel", "channel_id", link.MattermostChannelID, "team_id", link.MattermostTeamID, "teams_channel_id", link.MSTeamsChannel, "teams_team_id", link.MSTeamsTeam)

					// Create channel subscription for the linked channel
					m.recreateChannelSubscription(mmSubscription.SubscriptionID, mmSubscription.TeamID, mmSubscription.ChannelID, m.webhookSecret, false)
					<-ws
					return

				case shouldRefresh(mmSubscription.ExpiresOn):
					if isExpired(mmSubscription.ExpiresOn) {
						// In the future, this won't need to be an error if
						// we can resync, but for now notify a human.
						m.api.LogError("Subscription for channel expired", "channel_id", link.MattermostChannelID, "team_id", link.MattermostTeamID, "teams_channel_id", link.MSTeamsChannel, "teams_team_id", link.MSTeamsTeam)
					}

					m.api.LogInfo("Refreshing subscription for channel", "channel_id", link.MattermostChannelID, "team_id", link.MattermostTeamID, "teams_channel_id", link.MSTeamsChannel, "teams_team_id", link.MSTeamsTeam)

					if err := m.refreshSubscription(mmSubscription.SubscriptionID); err != nil {
						m.api.LogWarn("Failed to refresh channel subscription, recreating instead", "channel_id", link.MattermostChannelID, "team_id", link.MattermostTeamID, "teams_channel_id", link.MSTeamsChannel, "teams_team_id", link.MSTeamsTeam, "error", err.Error())
						m.recreateChannelSubscription(mmSubscription.SubscriptionID, mmSubscription.TeamID, mmSubscription.ChannelID, mmSubscription.Secret, true)
					}
				}
			} else {
				m.api.LogInfo("Creating subscription for channel", "channel_id", link.MattermostChannelID, "team_id", link.MattermostTeamID, "teams_channel_id", link.MSTeamsChannel, "teams_team_id", link.MSTeamsTeam)

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

// recreateChatSubscription deletes an existing getAllMessages subscription for a given user (if
// one exists) and recreates it, observing the event as a metric and recording the new subscription
// in the database.
func (m *Monitor) recreateChatSubscription(subscriptionID, userID, secret string) error {
	if err := m.client.DeleteSubscription(subscriptionID); err != nil {
		m.api.LogWarn("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err.Error())
	}

	newSubscription, err := m.client.SubscribeToUserChats(userID, m.baseURL, m.webhookSecret, !m.useEvaluationAPI, "")
	if err != nil {
		return err
	}

	return m.store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: newSubscription.ID, UserID: userID, Secret: secret, ExpiresOn: newSubscription.ExpiresOn})
}

// recreateChannelSubscription deletes an existing channel subscription (if it exists and
// deleteFromClient is true) and recreates it, observing the event as a metric and recording the
// new subscription in the database.
func (m *Monitor) recreateChannelSubscription(subscriptionID, teamID, channelID, secret string, deleteFromClient bool) {
	if deleteFromClient && subscriptionID != "" {
		if err := m.client.DeleteSubscription(subscriptionID); err != nil {
			m.api.LogWarn("Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", err.Error())
		}
	}

	newSubscription, err := m.client.SubscribeToChannel(teamID, channelID, m.baseURL, m.webhookSecret, "")
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

	if err = m.store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: newSubscription.ID, TeamID: teamID, ChannelID: channelID, Secret: secret, ExpiresOn: newSubscription.ExpiresOn}); err != nil {
		m.api.LogError("Unable to store new subscription in DB", "subscription_id", newSubscription.ID, "error", err.Error())
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
