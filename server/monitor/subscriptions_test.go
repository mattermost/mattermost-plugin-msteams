package monitor

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	mocksClient "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	mocksStore "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/stretchr/testify/require"
)

func TestMonitorCheckGlobalSubscriptions(t *testing.T) {
	newExpiresOn := time.Now().Add(100 * time.Minute)
	mockGlobalSubscription := storemodels.GlobalSubscription{SubscriptionID: "test-id", Type: "allChats", Secret: "webhook-secret", ExpiresOn: newExpiresOn}
	allChatsSubscription := &msteams.Subscription{
		ID:        "test-id",
		ExpiresOn: newExpiresOn,
	}
	for _, testCase := range []struct {
		description            string
		setupClient            func(*mocksClient.Client)
		setupAPI               func(*plugintest.API)
		setupStore             func(*mocksStore.Store)
		msteamsSubscriptionMap map[string]*msteams.Subscription
		allChatsSubscription   *msteams.Subscription
	}{
		{
			description: "Fail to get global subscription list",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for global subscriptions").Times(1)
				mockAPI.On("LogError", "Unable to get the chat subscriptions from store", "error", "failed to get global subscription list").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptions").Return(nil, errors.New("failed to get global subscription list")).Times(1)
			},
		},
		{
			description:          "Empty list of subscriptions, but subscription present on MS Teams",
			allChatsSubscription: allChatsSubscription,
			setupClient:          func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for global subscriptions").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptions").Return([]*storemodels.GlobalSubscription{}, nil).Times(1)
				store.On("SaveGlobalSubscription", mockGlobalSubscription).Return(nil).Times(1)
			},
		},
		{
			description: "Empty list of subscriptions, but subscription not present on MS Teams",
			setupClient: func(client *mocksClient.Client) {
				client.On("SubscribeToChats", "base-url", "webhook-secret", true).Return(allChatsSubscription, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for global subscriptions").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptions").Return([]*storemodels.GlobalSubscription{}, nil).Times(1)
				store.On("SaveGlobalSubscription", mockGlobalSubscription).Return(nil).Times(1)
			},
		},
		{
			description: "Subscription not present on MS Teams",
			setupClient: func(client *mocksClient.Client) {
				client.On("SubscribeToChats", "base-url", "webhook-secret", true).Return(allChatsSubscription, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for global subscriptions").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptions").Return([]*storemodels.GlobalSubscription{{SubscriptionID: "test", Type: "allChats", Secret: "webhook-secret", ExpiresOn: time.Now().Add(10 * time.Second)}}, nil).Times(1)
				store.On("DeleteSubscription", "test").Return(nil).Times(1)
				store.On("SaveGlobalSubscription", mockGlobalSubscription).Return(nil).Times(1)
			},
		},
		{
			description: "Unable to refresh the subscription",
			msteamsSubscriptionMap: map[string]*msteams.Subscription{
				"test-id": allChatsSubscription,
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test-id").Return(nil, errors.New("unable to refresh the subscription")).Times(1)
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToChats", "base-url", "webhook-secret", true).Return(allChatsSubscription, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for global subscriptions").Times(1)
				mockAPI.On("LogDebug", "Unable to refresh all chats subscription", "error", "unable to refresh the subscription").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptions").Return([]*storemodels.GlobalSubscription{{SubscriptionID: "test-id", Type: "allChats", Secret: "webhook-secret", ExpiresOn: time.Now().Add(10 * time.Second)}}, nil).Times(1)
				store.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				store.On("SaveGlobalSubscription", mockGlobalSubscription).Return(nil).Times(1)
			},
		},
		{
			description: "Not expired subscription",
			msteamsSubscriptionMap: map[string]*msteams.Subscription{
				"test-id": allChatsSubscription,
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test-id").Return(&newExpiresOn, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for global subscriptions").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListGlobalSubscriptions").Return([]*storemodels.GlobalSubscription{{SubscriptionID: "test-id", Type: "allChats", Secret: "webhook-secret", ExpiresOn: time.Now().Add(3 * time.Minute)}}, nil).Times(1)
				store.On("UpdateSubscriptionExpiresOn", "test-id", newExpiresOn).Return(nil).Times(1)
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

			monitor.checkGlobalSubscriptions(testCase.msteamsSubscriptionMap, testCase.allChatsSubscription)
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

	channelSubscription := &msteams.Subscription{
		ID:        "test",
		ChannelID: "channel-id",
		TeamID:    "team-id",
		ExpiresOn: newExpiresOn,
	}
	for _, testCase := range []struct {
		description             string
		msteamsSubscriptionsMap map[string]*msteams.Subscription
		setupClient             func(*mocksClient.Client)
		setupAPI                func(*plugintest.API)
		setupStore              func(*mocksStore.Store)
	}{
		{
			description: "Failed to get channel links",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for channels subscriptions").Times(1)
				mockAPI.On("LogError", "Unable to list channel links from DB", "error", "failed to get channel links").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return(nil, errors.New("failed to get channel links")).Times(1)
			},
		},
		{
			description: "Failed to get channel subscriptions",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for channels subscriptions").Times(1)
				mockAPI.On("LogError", "Unable to get the channel subscriptions", "error", "failed to get channel subscriptions").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return([]storemodels.ChannelLink{channelLink}, nil).Times(1)
				store.On("ListChannelSubscriptions").Return(nil, errors.New("failed to get channel subscriptions")).Times(1)
			},
		},
		{
			description: "Empty list of links",
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for channels subscriptions").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return([]storemodels.ChannelLink{}, nil).Times(1)
				store.On("ListChannelSubscriptions").Return([]*storemodels.ChannelSubscription{}, nil).Times(1)
			},
		},
		{
			description: "Empty list of subscriptions",
			setupClient: func(client *mocksClient.Client) {
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret").Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for channels subscriptions").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return([]storemodels.ChannelLink{channelLink}, nil).Times(1)
				store.On("ListChannelSubscriptions").Return([]*storemodels.ChannelSubscription{}, nil).Times(1)
				store.On("BeginTx").Return(&sql.Tx{}, nil).Times(1)
				store.On("SaveChannelSubscription", storemodels.ChannelSubscription{SubscriptionID: "new-id", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}, &sql.Tx{}).Return(nil).Times(1)
				store.On("CommitTx", &sql.Tx{}).Return(nil).Times(1)
			},
		},
		{
			description: "Subscription found on Mattermost but not on MS Teams",
			setupClient: func(client *mocksClient.Client) {
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret").Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for channels subscriptions").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return([]storemodels.ChannelLink{channelLink}, nil).Times(1)
				store.On("ListChannelSubscriptions").Return([]*storemodels.ChannelSubscription{}, nil).Times(1)
				store.On("BeginTx").Return(&sql.Tx{}, nil).Times(1)
				store.On("SaveChannelSubscription", storemodels.ChannelSubscription{SubscriptionID: "new-id", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}, &sql.Tx{}).Return(nil).Times(1)
				store.On("CommitTx", &sql.Tx{}).Return(nil).Times(1)
			},
		},
		{
			description: "Unable to refresh the subscription",
			msteamsSubscriptionsMap: map[string]*msteams.Subscription{
				"test": channelSubscription,
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test").Return(nil, errors.New("unable to refresh the subscription")).Times(1)
				client.On("DeleteSubscription", "test").Return(nil).Times(1)
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret").Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for channels subscriptions").Times(1)
				mockAPI.On("LogDebug", "Unable to refresh channel subscription", "error", "unable to refresh the subscription").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return([]storemodels.ChannelLink{channelLink}, nil).Times(1)
				store.On("ListChannelSubscriptions").Return([]*storemodels.ChannelSubscription{{SubscriptionID: "test", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: time.Now().Add(3 * time.Minute)}}, nil).Times(1)
				store.On("DeleteSubscription", "test").Return(nil).Times(1)
				store.On("BeginTx").Return(&sql.Tx{}, nil).Times(1)
				store.On("SaveChannelSubscription", storemodels.ChannelSubscription{SubscriptionID: "new-id", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}, &sql.Tx{}).Return(nil).Times(1)
				store.On("CommitTx", &sql.Tx{}).Return(nil).Times(1)
			},
		},
		{
			description: "Not expired subscription",
			msteamsSubscriptionsMap: map[string]*msteams.Subscription{
				"test": channelSubscription,
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test").Return(&newExpiresOn, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Checking for channels subscriptions").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("ListChannelLinks").Return([]storemodels.ChannelLink{channelLink}, nil).Times(1)
				store.On("ListChannelSubscriptions").Return([]*storemodels.ChannelSubscription{{SubscriptionID: "test", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: time.Now()}}, nil).Times(1)
				store.On("UpdateSubscriptionExpiresOn", "test", newExpiresOn).Return(nil).Times(1)
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
// 				mockAPI.On("LogDebug", "Checking for chats subscriptions").Times(1)
// 				mockAPI.On("LogError", "Unable to get the chat subscriptions", "error", mock.Anything).Times(1)
// 			},
// 			setupStore: func(store *mocksStore.Store) {
// 				store.On("ListChatSubscriptionsToCheck").Return(nil, errors.New("test"))
// 			},
// 		},
// 		{
// 			description: "Empty list of subscriptions",
// 			setupClient: func(client *mocksClient.Client) {},
// 			setupAPI: func(mockAPI *plugintest.API) {
// 				mockAPI.On("LogDebug", "Checking for chats subscriptions").Times(1)
// 				mockAPI.On("LogDebug", "Refreshing chats subscriptions", "count", 0).Times(1)
// 			},
// 			setupStore: func(store *mocksStore.Store) {
// 				store.On("ListChatSubscriptionsToCheck").Return([]storemodels.ChatSubscription{}, nil)
// 			},
// 		},
// 		{
// 			description: "Expired subscription",
// 			setupClient: func(client *mocksClient.Client) {
// 				client.On("DeleteSubscription", "test").Return(nil)
// 				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true).Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil)
// 			},
// 			setupAPI: func(mockAPI *plugintest.API) {
// 				mockAPI.On("LogDebug", "Checking for chats subscriptions").Times(1)
// 				mockAPI.On("LogDebug", "Refreshing chats subscriptions", "count", 1).Times(1)
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
// 				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true).Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil)
// 			},
// 			setupAPI: func(mockAPI *plugintest.API) {
// 				mockAPI.On("LogDebug", "Checking for chats subscriptions").Times(1)
// 				mockAPI.On("LogDebug", "Refreshing chats subscriptions", "count", 1).Times(1)
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
// 				mockAPI.On("LogDebug", "Checking for chats subscriptions").Times(1)
// 				mockAPI.On("LogDebug", "Refreshing chats subscriptions", "count", 1).Times(1)
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
	}{
		{
			description:    "Failed to delete previous subscription",
			subscriptionID: "test-id",
			secret:         "webhook-secret",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(errors.New("failed to delete previous subscription")).Times(1)
				client.On("SubscribeToChats", "base-url", "webhook-secret", true).Return(nil, errors.New("test")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", "failed to delete previous subscription").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {},
		},
		{
			description:    "Failed to subscribe to chats",
			subscriptionID: "test-id",
			secret:         "webhook-secret",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToChats", "base-url", "webhook-secret", true).Return(nil, errors.New("test")).Times(1)
			},
			setupAPI:   func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {},
		},
		{
			description:    "Failed to save the global subscription in the database",
			subscriptionID: "test-id",
			secret:         "webhook-secret",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToChats", "base-url", "webhook-secret", true).Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Unable to delete old global subscription from DB", "subscriptionID", "test-id", "error", "error in deleting subscription from store").Return()
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("DeleteSubscription", "test-id").Return(errors.New("error in deleting subscription from store"))
				store.On("SaveGlobalSubscription", storemodels.GlobalSubscription{SubscriptionID: "new-id", Type: "allChats", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(errors.New("test"))
			},
		},
		{
			description:    "subscription recreated",
			subscriptionID: "test-id",
			secret:         "webhook-secret",
			setupClient: func(client *mocksClient.Client) {
				client.On("DeleteSubscription", "test-id").Return(nil).Times(1)
				client.On("SubscribeToChats", "base-url", "webhook-secret", true).Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("DeleteSubscription", "test-id").Return(nil).Once()
				store.On("SaveGlobalSubscription", storemodels.GlobalSubscription{SubscriptionID: "new-id", Type: "allChats", Secret: "webhook-secret", ExpiresOn: newExpiresOn}).Return(nil)
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

func TestMonitorRecreateChannelSubscription(t *testing.T) {
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
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret").Return(nil, errors.New("failed to subscribe to channel")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", "failed to delete previous subscription").Times(1)
				mockAPI.On("LogError", "Unable to create new subscription for the channel", "channelID", "channel-id", "error", "failed to subscribe to channel").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {},
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
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret").Return(nil, errors.New("failed to subscribe to channel")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogError", "Unable to create new subscription for the channel", "channelID", "channel-id", "error", "failed to subscribe to channel").Times(1)
			},
			setupStore: func(store *mocksStore.Store) {},
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
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret").Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Unable to delete old channel subscription from DB", "subscriptionID", "test-id", "error", "error in deleting subscription from store").Return()
				mockAPI.On("LogError", "Unable to store new subscription in DB", "subscriptionID", "new-id", "error", "failed to save the channel subscription in the database").Return().Times(1)
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("DeleteSubscription", "test-id").Return(errors.New("error in deleting subscription from store"))
				store.On("BeginTx").Return(&sql.Tx{}, nil).Times(1)
				store.On("SaveChannelSubscription", storemodels.ChannelSubscription{SubscriptionID: "new-id", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}, &sql.Tx{}).Return(errors.New("failed to save the channel subscription in the database")).Times(1)
				store.On("RollbackTx", &sql.Tx{}).Return(nil).Times(1)
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
				client.On("SubscribeToChannel", "team-id", "channel-id", "base-url", "webhook-secret").Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {
				store.On("DeleteSubscription", "test-id").Return(nil)
				store.On("BeginTx").Return(&sql.Tx{}, nil).Times(1)
				store.On("SaveChannelSubscription", storemodels.ChannelSubscription{SubscriptionID: "new-id", TeamID: "team-id", ChannelID: "channel-id", Secret: "webhook-secret", ExpiresOn: newExpiresOn}, &sql.Tx{}).Return(nil)
				store.On("CommitTx", &sql.Tx{}).Return(nil).Times(1)
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
				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true).Return(nil, errors.New("test")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogDebug", "Unable to delete old subscription, maybe it doesn't exist anymore in the server", "error", "failed to delete previous subscription").Times(1)
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
				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true).Return(nil, errors.New("test")).Times(1)
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
				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true).Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
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
				client.On("SubscribeToUserChats", "user-id", "base-url", "webhook-secret", true).Return(&msteams.Subscription{ID: "new-id", ExpiresOn: newExpiresOn}, nil).Times(1)
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
			monitor := New(client, store, mockAPI, "base-url", "webhook-secret", false)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)
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
	}{
		{
			description:    "Failed to refresh the subscription",
			subscriptionID: "test-id",
			expectsError:   true,
			setupClient: func(client *mocksClient.Client) {
				client.On("RefreshSubscription", "test-id").Return(nil, errors.New("test")).Times(1)
			},
			setupAPI:   func(mockAPI *plugintest.API) {},
			setupStore: func(store *mocksStore.Store) {},
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
