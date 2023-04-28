package monitor

import (
	"errors"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	mocksClient "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	mocksStore "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/stretchr/testify/mock"
)

func TestMonitorCheckGlobalSubscriptions(t *testing.T) {
	newExpiresOn := time.Now().Add(100 * time.Minute)
	for _, testCase := range []struct {
		description string
		setupClient func(*mocksClient.Client)
		setupAPI    func(*plugintest.API)
		setupStore  func(*mocksStore.Store)
	}{
		{
			description: "Fail to get global subscription list",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for global subscriptions").Times(1)
				mockAPI.On("LogError", "Unable to get the global subscriptions", "error", mock.Anything).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptionsToCheck").Return(nil, errors.New("test"))
			},
		},
		{
			description: "Empty list of subscriptions",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for global subscriptions").Times(1)
				mockAPI.On("LogDebug", "Refreshing global subscriptions", "count", 0).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptionsToCheck").Return([]storemodels.GlobalSubscription{}, nil)
			},
		},
		{
			description: "Expired subscription",
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test").Return(nil)
				client.On("SubscribeToChannels", "base-url", "webhook-secret", true).Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for global subscriptions").Times(1)
				mockAPI.On("LogDebug", "Refreshing global subscriptions", "count", 1).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptionsToCheck").Return([]storemodels.GlobalSubscription{{SubscriptionID: "test", Type: "allChannels", Secret: "webhook-secret", ExpiresOn: time.Now().Add(-1 * time.Minute)}}, nil)
				store.On("SaveGlobalSubscription", storemodels.GlobalSubscription{SubscriptionID: "new-id", Type: "allChannels", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil)
			},
		},
		{
			description: "Almost expired subscription",
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test").Return(nil)
				client.On("SubscribeToChannels", "base-url", "webhook-secret", true).Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for global subscriptions").Times(1)
				mockAPI.On("LogDebug", "Refreshing global subscriptions", "count", 1).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptionsToCheck").Return([]storemodels.GlobalSubscription{{SubscriptionID: "test", Type: "allChannels", Secret: "webhook-secret", ExpiresOn: time.Now().Add(10 * time.Second)}}, nil)
				store.On("SaveGlobalSubscription", storemodels.GlobalSubscription{SubscriptionID: "new-id", Type: "allChannels", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil)
			},
		},
		{
			description: "Not expired subscription",
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test").Return(&newExpiresOn, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for global subscriptions").Times(1)
				mockAPI.On("LogDebug", "Refreshing global subscriptions", "count", 1).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptionsToCheck").Return([]storemodels.GlobalSubscription{{SubscriptionID: "test", Type: "allChannels", Secret: "webhook-secret", ExpiresOn: time.Now().Add(3 * time.Minute)}}, nil)
				store.On("UpdateSubscriptionExpiresOn", "test", newExpiresOn).Return(nil)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			monitor := New(client, store, mockAPI, "base-url", "webhook-secret", false)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(store)

			monitor.checkGlobalSubscriptions()
			store.AssertExpectations(t)
			mockAPI.AssertExpectations(t)
			client.AssertExpectations(t)
		})
	}
}

func TestMonitorCheckChannelSubscriptions(t *testing.T) {
	newExpiresOn := time.Now().Add(100 * time.Minute)
	for _, testCase := range []struct {
		description string
		setupClient func(*mocksClient.Client)
		setupAPI    func(*plugintest.API)
		setupStore  func(*mocksStore.Store)
	}{
		{
			description: "Fail to get channels subscription list",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for channels subscriptions").Times(1)
				mockAPI.On("LogError", "Unable to get the channel subscriptions", "error", mock.Anything).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelSubscriptionsToCheck").Return(nil, errors.New("test"))
			},
		},
		{
			description: "Empty list of subscriptions",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for channels subscriptions").Times(1)
				mockAPI.On("LogDebug", "Refreshing channels subscriptions", "count", 0).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelSubscriptionsToCheck").Return([]storemodels.ChannelSubscription{}, nil)
			},
		},
		{
			description: "Expired subscription",
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test").Return(nil)
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret").Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for channels subscriptions").Times(1)
				mockAPI.On("LogDebug", "Refreshing channels subscriptions", "count", 1).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelSubscriptionsToCheck").Return([]storemodels.ChannelSubscription{{SubscriptionID: "test", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: time.Now().Add(-1 * time.Minute)}}, nil)
				store.On("SaveChannelSubscription", storemodels.ChannelSubscription{SubscriptionID: "new-id", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil)
			},
		},
		{
			description: "Almost expired subscription",
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test").Return(nil)
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret").Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for channels subscriptions").Times(1)
				mockAPI.On("LogDebug", "Refreshing channels subscriptions", "count", 1).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelSubscriptionsToCheck").Return([]storemodels.ChannelSubscription{{SubscriptionID: "test", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: time.Now().Add(10 * time.Second)}}, nil)
				store.On("SaveChannelSubscription", storemodels.ChannelSubscription{SubscriptionID: "new-id", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil)
			},
		},
		{
			description: "Not expired subscription",
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test").Return(&newExpiresOn, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for channels subscriptions").Times(1)
				mockAPI.On("LogDebug", "Refreshing channels subscriptions", "count", 1).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelSubscriptionsToCheck").Return([]storemodels.ChannelSubscription{{SubscriptionID: "test", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: time.Now().Add(3 * time.Minute)}}, nil)
				store.On("UpdateSubscriptionExpiresOn", "test", newExpiresOn).Return(nil)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			monitor := New(client, store, mockAPI, "base-url", "webhook-secret", false)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(store)

			monitor.checkChannelsSubscriptions()
			store.AssertExpectations(t)
			mockAPI.AssertExpectations(t)
			client.AssertExpectations(t)
		})
	}
}

func TestMonitorCheckChatSubscriptions(t *testing.T) {
	newExpiresOn := time.Now().Add(100 * time.Minute)
	for _, testCase := range []struct {
		description string
		setupClient func(*mocksClient.Client)
		setupAPI    func(*plugintest.API)
		setupStore  func(*mocksStore.Store)
	}{
		{
			description: "Fail to get chats subscription list",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for chats subscriptions").Times(1)
				mockAPI.On("LogError", "Unable to get the chat subscriptions", "error", mock.Anything).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChatSubscriptionsToCheck").Return(nil, errors.New("test"))
			},
		},
		{
			description: "Empty list of subscriptions",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for chats subscriptions").Times(1)
				mockAPI.On("LogDebug", "Refreshing chats subscriptions", "count", 0).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChatSubscriptionsToCheck").Return([]storemodels.ChatSubscription{}, nil)
			},
		},
		{
			description: "Expired subscription",
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test").Return(nil)
				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true).Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for chats subscriptions").Times(1)
				mockAPI.On("LogDebug", "Refreshing chats subscriptions", "count", 1).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChatSubscriptionsToCheck").Return([]storemodels.ChatSubscription{{SubscriptionID: "test", UserID: "user-id", Secret: "webhook-secret", ExpiresOn: time.Now().Add(-1 * time.Minute)}}, nil)
				store.On("SaveChatSubscription", storemodels.ChatSubscription{SubscriptionID: "new-id", UserID: "user-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil)
			},
		},
		{
			description: "Almost expired subscription",
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test").Return(nil)
				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true).Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for chats subscriptions").Times(1)
				mockAPI.On("LogDebug", "Refreshing chats subscriptions", "count", 1).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChatSubscriptionsToCheck").Return([]storemodels.ChatSubscription{{SubscriptionID: "test", UserID: "user-id", Secret: "webhook-secret", ExpiresOn: time.Now().Add(10 * time.Second)}}, nil)
				store.On("SaveChatSubscription", storemodels.ChatSubscription{SubscriptionID: "new-id", UserID: "user-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil)
			},
		},
		{
			description: "Not expired subscription",
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test").Return(&newExpiresOn, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for chats subscriptions").Times(1)
				mockAPI.On("LogDebug", "Refreshing chats subscriptions", "count", 1).Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChatSubscriptionsToCheck").Return([]storemodels.ChatSubscription{{SubscriptionID: "test", UserID: "user-id", Secret: "webhook-secret", ExpiresOn: time.Now().Add(3 * time.Minute)}}, nil)
				store.On("UpdateSubscriptionExpiresOn", "test", newExpiresOn).Return(nil)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			monitor := New(client, store, mockAPI, "base-url", "webhook-secret", false)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(store)

			monitor.checkChatsSubscriptions()
			store.AssertExpectations(t)
			mockAPI.AssertExpectations(t)
			client.AssertExpectations(t)
		})
	}
}
