package monitor

import (
	"errors"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	mocksMetrics "github.com/mattermost/mattermost-plugin-msteams/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	mocksClient "github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	mocksStore "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/require"
)

func TestMonitorCheckGlobalSubscriptions(t *testing.T) {
	newExpiresOn := time.Now().Add(100 * time.Minute)
	mockGlobalSubscription := storemodels.GlobalSubscription{SubscriptionID: "test-id", Type: "allChats", Secret: "webhook-secret", ExpiresOn: newExpiresOn}
	allChatsSubscription := &clientmodels.Subscription{
		ID:        "test-id",
		ExpiresOn: newExpiresOn,
	}
	for _, testCase := range []struct {
		description            string
		setupClient            func(*mocksClient.Client)
		setupAPI               func(*plugintest.API)
		setupStore             func(*mocksStore.Store)
		setupMetrics           func(*mocksMetrics.Metrics)
		msteamsSubscriptionMap map[string]*clientmodels.Subscription
		allChatsSubscription   *clientmodels.Subscription
	}{
		{
			description: "Fail to get global subscription list",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptions").Return(nil, errors.New("failed to get global subscription list")).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description:          "Empty list of subscriptions, but subscription present on MS Teams",
			allChatsSubscription: allChatsSubscription,
			setupClient:          func(client *mocksClient.Client) {},
			setupAPI:             func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptions").Return([]*storemodels.GlobalSubscription{}, nil).Times(1)
				store.On("SaveGlobalSubscription", mockGlobalSubscription).Return(nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Empty list of subscriptions, but subscription not present on MS Teams",
			setupClient: func(client *mocksClient.Client) {
				client.On("SubscribeToChats", "base-url", "webhook-secret", true, "").Return(allChatsSubscription, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptions").Return([]*storemodels.GlobalSubscription{}, nil).Times(1)
				store.On("SaveGlobalSubscription", mockGlobalSubscription).Return(nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionConnected).Times(1)
			},
		},
		{
			description: "Subscription not present on MS Teams",
			setupClient: func(client *mocksClient.Client) {
				client.On("SubscribeToChats", "base-url", "webhook-secret", true, "").Return(allChatsSubscription, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptions").Return([]*storemodels.GlobalSubscription{{SubscriptionID: "test", Type: "allChats", Secret: "webhook-secret", ExpiresOn: time.Now().Add(10 * time.Second)}}, nil).Times(1)
				store.On("DeleteSubscription", "test").Return(nil).Times(1)
				store.On("SaveGlobalSubscription", mockGlobalSubscription).Return(nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionConnected).Times(1)
			},
		},
		{
			description: "Unable to refresh the subscription",
			msteamsSubscriptionMap: map[string]*clientmodels.Subscription{
				"test-id": allChatsSubscription,
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test-id").Return(nil, errors.New("unable to refresh the subscription")).Times(1)
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToChats", "base-url", "webhook-secret", true, "").Return(allChatsSubscription, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptions").Return([]*storemodels.GlobalSubscription{{SubscriptionID: "test-id", Type: "allChats", Secret: "webhook-secret", ExpiresOn: time.Now().Add(10 * time.Second)}}, nil).Times(1)
				store.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				store.On("SaveGlobalSubscription", mockGlobalSubscription).Return(nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionReconnected).Times(1)
			},
		},
		{
			description: "Not expired subscription",
			msteamsSubscriptionMap: map[string]*clientmodels.Subscription{
				"test-id": allChatsSubscription,
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test-id").Return(&newExpiresOn, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptions").Return([]*storemodels.GlobalSubscription{{SubscriptionID: "test-id", Type: "allChats", Secret: "webhook-secret", ExpiresOn: time.Now().Add(3 * time.Minute)}}, nil).Times(1)
				store.On("UpdateSubscriptionExpiresOn", "test-id", newExpiresOn).Return(nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionRefreshed).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			mockmetrics := mocksMetrics.NewMetrics(t)
			monitor := New(client, store, mockAPI, mockmetrics, "base-url", "webhook-secret", false, "", false, true, false)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testutils.MockLogs(mockAPI)
			testCase.setupStore(store)
			testCase.setupMetrics(mockmetrics)

			monitor.checkGlobalChatsSubscription(testCase.msteamsSubscriptionMap, testCase.allChatsSubscription)
			store.AssertExpectations(t)
			mockAPI.AssertExpectations(t)
			client.AssertExpectations(t)
		})
	}
}

func TestMonitorCheckChannelSubscriptions(t *testing.T) {
	newExpiresOn := time.Now().Add(100 * time.Minute)
	channelLink := storemodels.ChannelLink{
		MSTeamsTeam:         "team-id",
		MSTeamsChannel:      "channel-id",
		MattermostTeamID:    "mm-team-id",
		MattermostChannelID: "mm-channel-id",
	}

	channelSubscription := &clientmodels.Subscription{
		ID:        "test",
		ChannelID: "channel-id",
		TeamID:    "team-id",
		ExpiresOn: newExpiresOn,
	}
	for _, testCase := range []struct {
		description             string
		msteamsSubscriptionsMap map[string]*clientmodels.Subscription
		setupClient             func(*mocksClient.Client)
		setupAPI                func(*plugintest.API)
		setupStore              func(*mocksStore.Store)
		setupMetrics            func(mockmetrics *mocksMetrics.Metrics)
	}{
		{
			description: "Failed to get channel links",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return(nil, errors.New("failed to get channel links")).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Failed to get channel subscriptions",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return([]storemodels.ChannelLink{channelLink}, nil).Times(1)
				store.On("ListChannelSubscriptions").Return(nil, errors.New("failed to get channel subscriptions")).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Empty list of links",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI:    func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return([]storemodels.ChannelLink{}, nil).Times(1)
				store.On("ListChannelSubscriptions").Return([]*storemodels.ChannelSubscription{}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description: "Empty list of subscriptions",
			setupClient: func(client *mocksClient.Client) {
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret", "").Return(&clientmodels.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return([]storemodels.ChannelLink{channelLink}, nil).Times(1)
				store.On("ListChannelSubscriptions").Return([]*storemodels.ChannelSubscription{}, nil).Times(1)
				store.On("SaveChannelSubscription", storemodels.ChannelSubscription{SubscriptionID: "new-id", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionReconnected).Times(1)
			},
		},
		{
			description: "Subscription found on Mattermost but not on MS Teams",
			setupClient: func(client *mocksClient.Client) {
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret", "").Return(&clientmodels.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return([]storemodels.ChannelLink{channelLink}, nil).Times(1)
				store.On("ListChannelSubscriptions").Return([]*storemodels.ChannelSubscription{}, nil).Times(1)
				store.On("SaveChannelSubscription", storemodels.ChannelSubscription{SubscriptionID: "new-id", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionReconnected).Times(1)
			},
		},
		{
			description: "Unable to refresh the subscription",
			msteamsSubscriptionsMap: map[string]*clientmodels.Subscription{
				"test": channelSubscription,
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test").Return(nil, errors.New("unable to refresh the subscription")).Times(1)
				client.On("DeleteSubscription", "test").Return(nil).Times(1)
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret", "").Return(&clientmodels.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return([]storemodels.ChannelLink{channelLink}, nil).Times(1)
				store.On("ListChannelSubscriptions").Return([]*storemodels.ChannelSubscription{{SubscriptionID: "test", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: time.Now().Add(3 * time.Minute)}}, nil).Times(1)
				store.On("DeleteSubscription", "test").Return(nil).Times(1)
				store.On("SaveChannelSubscription", storemodels.ChannelSubscription{SubscriptionID: "new-id", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionReconnected).Times(1)
			},
		},
		{
			description: "Not expired subscription",
			msteamsSubscriptionsMap: map[string]*clientmodels.Subscription{
				"test": channelSubscription,
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test").Return(&newExpiresOn, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return([]storemodels.ChannelLink{channelLink}, nil).Times(1)
				store.On("ListChannelSubscriptions").Return([]*storemodels.ChannelSubscription{{SubscriptionID: "test", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: time.Now()}}, nil).Times(1)
				store.On("UpdateSubscriptionExpiresOn", "test", newExpiresOn).Return(nil).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionRefreshed).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			mockmetrics := mocksMetrics.NewMetrics(t)
			monitor := New(client, store, mockAPI, mockmetrics, "base-url", "webhook-secret", false, "", false, true, false)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testutils.MockLogs(mockAPI)
			testCase.setupStore(store)
			testCase.setupMetrics(mockmetrics)

			monitor.checkChannelsSubscriptions(testCase.msteamsSubscriptionsMap)
			store.AssertExpectations(t)
			mockAPI.AssertExpectations(t)
			client.AssertExpectations(t)
		})
	}
}

// Commenting the below function as we are not creating any user type subscriptions
// func TestMonitorCheckChatSubscriptions(t *testing.T) {
// 	newExpiresOn := time.Now().Add(100 * time.Minute)
// 	for _, testCase := range []struct {
// 		description string
// 		setupClient func(*mocksClient.Client)
// 		setupAPI    func(*plugintest.API)
// 		setupStore  func(*mocksStore.Store)
// 	}{
// 		{
// 			description: "Fail to get chats subscription list",
// 			setupClient: func(client *mocksClient.Client) {},
// 			setupAPI: func(mockAPI *plugintest.API) {
// 			},
// 			setupStore: func(store *mocksStore.Store) {
// 				store.On("ListChatSubscriptionsToCheck").Return(nil, errors.New("test"))
// 			},
// 		},
// 		{
// 			description: "Empty list of subscriptions",
// 			setupClient: func(client *mocksClient.Client) {},
// 			setupAPI: func(mockAPI *plugintest.API) {
// 			},
// 			setupStore: func(store *mocksStore.Store) {
// 				store.On("ListChatSubscriptionsToCheck").Return([]storemodels.ChatSubscription{}, nil)
// 			},
// 		},
// 		{
// 			description: "Expired subscription",
// 			setupClient: func(client *mocksClient.Client) {
// 				client.On("DeleteSubscription", "test").Return(nil)
// 				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true).Return(&clientmodels.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil)
// 			},
// 			setupAPI: func(mockAPI *plugintest.API) {
// 			},
// 			setupStore: func(store *mocksStore.Store) {
// 				store.On("ListChatSubscriptionsToCheck").Return([]storemodels.ChatSubscription{{SubscriptionID: "test", UserID: "user-id", Secret: "webhook-secret", ExpiresOn: time.Now().Add(-1 * time.Minute)}}, nil)
// 				store.On("SaveChatSubscription", storemodels.ChatSubscription{SubscriptionID: "new-id", UserID: "user-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil)
// 			},
// 		},
// 		{
// 			description: "Almost expired subscription",
// 			setupClient: func(client *mocksClient.Client) {
// 				client.On("DeleteSubscription", "test").Return(nil)
// 				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true).Return(&clientmodels.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil)
// 			},
// 			setupAPI: func(mockAPI *plugintest.API) {
// 			},
// 			setupStore: func(store *mocksStore.Store) {
// 				store.On("ListChatSubscriptionsToCheck").Return([]storemodels.ChatSubscription{{SubscriptionID: "test", UserID: "user-id", Secret: "webhook-secret", ExpiresOn: time.Now().Add(10 * time.Second)}}, nil)
// 				store.On("SaveChatSubscription", storemodels.ChatSubscription{SubscriptionID: "new-id", UserID: "user-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil)
// 			},
// 		},
// 		{
// 			description: "Not expired subscription",
// 			setupClient: func(client *mocksClient.Client) {
// 				client.On("RefreshSubscription", "test").Return(&newExpiresOn, nil)
// 			},
// 			setupAPI: func(mockAPI *plugintest.API) {
// 			},
// 			setupStore: func(store *mocksStore.Store) {
// 				store.On("ListChatSubscriptionsToCheck").Return([]storemodels.ChatSubscription{{SubscriptionID: "test", UserID: "user-id", Secret: "webhook-secret", ExpiresOn: time.Now().Add(3 * time.Minute)}}, nil)
// 				store.On("UpdateSubscriptionExpiresOn", "test", newExpiresOn).Return(nil)
// 			},
// 		},
// 	} {
// 		t.Run(testCase.description, func(t *testing.T) {
// 			store := mocksStore.NewStore(t)
// 			mockAPI := &plugintest.API{}
// 			client := mocksClient.NewClient(t)
// 			monitor := New(client, store, mockAPI, "base-url", "webhook-secret", false)
// 			testCase.setupClient(client)
// 			testCase.setupAPI(mockAPI)
//			testutils.MockLogs(mockAPI)
// 			testCase.setupStore(store)

// 			monitor.checkChatsSubscriptions()
// 			store.AssertExpectations(t)
// 			mockAPI.AssertExpectations(t)
// 			client.AssertExpectations(t)
// 		})
// 	}
// }

func TestMonitorRecreateGlobalSubscription(t *testing.T) {
	newExpiresOn := time.Now().Add(100 * time.Minute)
	for _, testCase := range []struct {
		description    string
		subscriptionID string
		secret         string
		expectsError   bool
		setupClient    func(*mocksClient.Client)
		setupAPI       func(*plugintest.API)
		setupStore     func(*mocksStore.Store)
		setupMetrics   func(mockmetrics *mocksMetrics.Metrics)
	}{
		{
			description:    "Failed to delete previous subscription",
			subscriptionID: "test-id",
			secret:         "webhook-secret",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(errors.New("failed to delete previous subscription")).Times(1)
				client.On("SubscribeToChats", "base-url", "webhook-secret", true, "").Return(nil, errors.New("test")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description:    "Failed to subscribe to chats",
			subscriptionID: "test-id",
			secret:         "webhook-secret",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToChats", "base-url", "webhook-secret", true, "").Return(nil, errors.New("test")).Times(1)
			},
			setupAPI:     func(mockAPI *plugintest.API) {},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description:    "Failed to save the global subscription in the database",
			subscriptionID: "test-id",
			secret:         "webhook-secret",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToChats", "base-url", "webhook-secret", true, "").Return(&clientmodels.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("DeleteSubscription", "test-id").Return(errors.New("error in deleting subscription from store"))
				store.On("SaveGlobalSubscription", storemodels.GlobalSubscription{SubscriptionID: "new-id", Type: "allChats", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(errors.New("test"))
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionReconnected).Times(1)
			},
		},
		{
			description:    "subscription recreated",
			subscriptionID: "test-id",
			secret:         "webhook-secret",
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToChats", "base-url", "webhook-secret", true, "").Return(&clientmodels.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("DeleteSubscription", "test-id").Return(nil).Once()
				store.On("SaveGlobalSubscription", storemodels.GlobalSubscription{SubscriptionID: "new-id", Type: "allChats", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionReconnected).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			mockmetrics := mocksMetrics.NewMetrics(t)
			monitor := New(client, store, mockAPI, mockmetrics, "base-url", "webhook-secret", false, "", false, true, false)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testutils.MockLogs(mockAPI)
			testCase.setupStore(store)
			testCase.setupMetrics(mockmetrics)

			err := monitor.recreateGlobalSubscription(testCase.subscriptionID, testCase.secret)
			if testCase.expectsError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			store.AssertExpectations(t)
			mockAPI.AssertExpectations(t)
			client.AssertExpectations(t)
		})
	}
}

func TestRecreateChannelSubscription(t *testing.T) {
	newExpiresOn := time.Now().Add(100 * time.Minute)
	for _, testCase := range []struct {
		description    string
		subscriptionID string
		teamID         string
		channelID      string
		secret         string
		expectsError   bool
		setupClient    func(*mocksClient.Client)
		setupAPI       func(*plugintest.API)
		setupStore     func(*mocksStore.Store)
		setupMetrics   func(mockmetrics *mocksMetrics.Metrics)
	}{
		{
			description:    "Failed to delete previous subscription",
			subscriptionID: "test-id",
			teamID:         "team-id",
			channelID:      "channel-id",
			secret:         "webhook-secret",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(errors.New("failed to delete previous subscription")).Times(1)
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret", "").Return(nil, errors.New("failed to subscribe to channel")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description:    "Failed to subscribe to channel",
			subscriptionID: "test-id",
			teamID:         "team-id",
			channelID:      "channel-id",
			secret:         "webhook-secret",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret", "").Return(nil, errors.New("failed to subscribe to channel")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description:    "Failed to save the channel subscription in the database",
			subscriptionID: "test-id",
			teamID:         "team-id",
			channelID:      "channel-id",
			secret:         "webhook-secret",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret", "").Return(&clientmodels.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("DeleteSubscription", "test-id").Return(errors.New("error in deleting subscription from store"))
				store.On("SaveChannelSubscription", storemodels.ChannelSubscription{SubscriptionID: "new-id", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(errors.New("failed to save the channel subscription in the database")).Times(1)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionReconnected).Times(1)
			},
		},
		{
			description:    "subscription recreated",
			subscriptionID: "test-id",
			teamID:         "team-id",
			channelID:      "channel-id",
			secret:         "webhook-secret",
			expectsError:   false,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret", "").Return(&clientmodels.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("DeleteSubscription", "test-id").Return(nil)
				store.On("SaveChannelSubscription", storemodels.ChannelSubscription{SubscriptionID: "new-id", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionReconnected).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			mockmetrics := mocksMetrics.NewMetrics(t)
			monitor := New(client, store, mockAPI, mockmetrics, "base-url", "webhook-secret", false, "", false, true, false)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testutils.MockLogs(mockAPI)
			testCase.setupStore(store)
			testCase.setupMetrics(mockmetrics)

			monitor.recreateChannelSubscription(testCase.subscriptionID, testCase.teamID, testCase.channelID, testCase.secret, true)

			store.AssertExpectations(t)
			mockAPI.AssertExpectations(t)
			client.AssertExpectations(t)
		})
	}
}

func TestMonitorRecreateChatSubscription(t *testing.T) {
	newExpiresOn := time.Now().Add(100 * time.Minute)
	for _, testCase := range []struct {
		description    string
		subscriptionID string
		userID         string
		secret         string
		expectsError   bool
		setupClient    func(*mocksClient.Client)
		setupAPI       func(*plugintest.API)
		setupStore     func(*mocksStore.Store)
	}{
		{
			description:    "Failed to delete previous subscription",
			subscriptionID: "test-id",
			userID:         "user-id",
			secret:         "webhook-secret",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(errors.New("failed to delete previous subscription")).Times(1)
				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true, "").Return(nil, errors.New("test")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {},
		},
		{
			description:    "Failed to subscribe to chats",
			subscriptionID: "test-id",
			userID:         "user-id",
			secret:         "webhook-secret",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true, "").Return(nil, errors.New("test")).Times(1)
			},
			setupAPI:   func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {},
		},
		{
			description:    "Failed to save the global subscription in the database",
			subscriptionID: "test-id",
			userID:         "user-id",
			secret:         "webhook-secret",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true, "").Return(&clientmodels.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("SaveChatSubscription", storemodels.ChatSubscription{SubscriptionID: "new-id", UserID: "user-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(errors.New("test"))
			},
		},
		{
			description:    "subscription recreated",
			subscriptionID: "test-id",
			userID:         "user-id",
			secret:         "webhook-secret",
			expectsError:   false,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true, "").Return(&clientmodels.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("SaveChatSubscription", storemodels.ChatSubscription{SubscriptionID: "new-id", UserID: "user-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			mockmetrics := mocksMetrics.NewMetrics(t)
			monitor := New(client, store, mockAPI, mockmetrics, "base-url", "webhook-secret", false, "", false, true, false)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testutils.MockLogs(mockAPI)
			testCase.setupStore(store)

			err := monitor.recreateChatSubscription(testCase.subscriptionID, testCase.userID, testCase.secret)
			if testCase.expectsError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			store.AssertExpectations(t)
			mockAPI.AssertExpectations(t)
			client.AssertExpectations(t)
		})
	}
}

func TestMonitorRefreshSubscription(t *testing.T) {
	newExpiresOn := time.Now().Add(100 * time.Minute)
	for _, testCase := range []struct {
		description    string
		subscriptionID string
		expectsError   bool
		setupClient    func(*mocksClient.Client)
		setupAPI       func(*plugintest.API)
		setupStore     func(*mocksStore.Store)
		setupMetrics   func(mockmetrics *mocksMetrics.Metrics)
	}{
		{
			description:    "Failed to refresh the subscription",
			subscriptionID: "test-id",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test-id").Return(nil, errors.New("test")).Times(1)
			},
			setupAPI:     func(mockAPI *plugintest.API) {},
			setupStore:   func(store *mocksStore.Store) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
		},
		{
			description:    "Failed to save the global subscription in the database",
			subscriptionID: "test-id",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test-id").Return(&newExpiresOn, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("UpdateSubscriptionExpiresOn", "test-id", newExpiresOn).Return(errors.New("test"))
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionRefreshed).Times(1)
			},
		},
		{
			description:    "subscription refreshed",
			subscriptionID: "test-id",
			expectsError:   false,
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test-id").Return(&newExpiresOn, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("UpdateSubscriptionExpiresOn", "test-id", newExpiresOn).Return(nil)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionRefreshed).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			mockmetrics := mocksMetrics.NewMetrics(t)
			monitor := New(client, store, mockAPI, mockmetrics, "base-url", "webhook-secret", false, "", false, true, false)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testutils.MockLogs(mockAPI)
			testCase.setupStore(store)
			testCase.setupMetrics(mockmetrics)

			err := monitor.refreshSubscription(testCase.subscriptionID)
			if testCase.expectsError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			store.AssertExpectations(t)
			mockAPI.AssertExpectations(t)
			client.AssertExpectations(t)
		})
	}
}
