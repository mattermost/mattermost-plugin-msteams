// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitorCheckGlobalChatsSubscription(t *testing.T) {
	th := setupTestHelper(t)

	setupLocalSubscription := func(th *testHelper, t *testing.T) storemodels.GlobalSubscription {
		subscription := storemodels.GlobalSubscription{
			SubscriptionID: model.NewId(),
			Type:           "allChats",
			ExpiresOn:      time.Now().Add(30 * time.Minute),
			Secret:         "webhooksecret",
			Certificate:    "",
		}
		err := th.p.store.SaveGlobalSubscription(subscription)
		require.NoError(t, err)

		return subscription
	}

	setupExpiringLocalSubscription := func(th *testHelper, t *testing.T) storemodels.GlobalSubscription {
		subscription := storemodels.GlobalSubscription{
			SubscriptionID: model.NewId(),
			Type:           "allChats",
			ExpiresOn:      time.Now().Add(1 * time.Minute),
			Secret:         "webhooksecret",
			Certificate:    "",
		}
		err := th.p.store.SaveGlobalSubscription(subscription)
		require.NoError(t, err)

		return subscription
	}

	// Check the store for global subscriptions and verify either no results or a single matching result.
	expectLocalSubscription := func(th *testHelper, t *testing.T, expectedSubscription *clientmodels.Subscription) {
		actualSubscriptions, err := th.p.store.ListGlobalSubscriptions()
		require.NoError(t, err)

		if expectedSubscription == nil {
			assert.Empty(t, actualSubscriptions)
		} else if assert.Len(t, actualSubscriptions, 1) {
			assert.Equal(t, "allChats", actualSubscriptions[0].Type)
			assert.Equal(t, expectedSubscription.ID, actualSubscriptions[0].SubscriptionID)
			assert.Equal(t, expectedSubscription.Type, actualSubscriptions[0].Type)
			assert.Equal(t, expectedSubscription.ExpiresOn.Round(time.Second), actualSubscriptions[0].ExpiresOn.Round(time.Second))
		}
	}

	t.Run("no local subscription, no remote subscription", func(t *testing.T) {
		th.Reset(t)

		var existingRemoteSubscription *clientmodels.Subscription

		newRemoteSubscription := &clientmodels.Subscription{
			ID:              model.NewId(),
			Type:            "allChats",
			ExpiresOn:       time.Now().Add(30 * time.Minute),
			NotificationURL: "http://example.com/plugins/com.mattermost.msteams-sync/",
		}
		th.appClientMock.On("SubscribeToChats", "http://example.com/plugins/com.mattermost.msteams-sync/", "webhooksecret", true, "").Return(newRemoteSubscription, nil).Times(1)

		th.p.monitor.checkGlobalChatsSubscription(existingRemoteSubscription)
		expectLocalSubscription(th, t, newRemoteSubscription)
	})

	t.Run("no local subscription, existing remote subscription", func(t *testing.T) {
		th.Reset(t)

		existingRemoteSubscription := &clientmodels.Subscription{
			ID:              model.NewId(),
			Type:            "allChats",
			ExpiresOn:       time.Now().Add(30 * time.Minute),
			NotificationURL: "http://example.com/plugins/com.mattermost.msteams-sync/",
		}
		th.appClientMock.On("DeleteSubscription", existingRemoteSubscription.ID).Return(nil).Times(1)

		newRemoteSubscription := &clientmodels.Subscription{
			ID:              model.NewId(),
			Type:            "allChats",
			ExpiresOn:       time.Now().Add(30 * time.Minute),
			NotificationURL: "http://example.com/plugins/com.mattermost.msteams-sync/",
		}
		th.appClientMock.On("SubscribeToChats", "http://example.com/plugins/com.mattermost.msteams-sync/", "webhooksecret", true, "").Return(newRemoteSubscription, nil).Times(1)

		th.p.monitor.checkGlobalChatsSubscription(existingRemoteSubscription)
		expectLocalSubscription(th, t, newRemoteSubscription)
	})

	t.Run("local subscription exists, no remote subscription", func(t *testing.T) {
		th.Reset(t)

		setupLocalSubscription(th, t)

		var existingRemoteSubscription *clientmodels.Subscription

		newRemoteSubscription := &clientmodels.Subscription{
			ID:              model.NewId(),
			Type:            "allChats",
			ExpiresOn:       time.Now().Add(30 * time.Minute),
			NotificationURL: "http://example.com/plugins/com.mattermost.msteams-sync/",
		}
		th.appClientMock.On("SubscribeToChats", "http://example.com/plugins/com.mattermost.msteams-sync/", "webhooksecret", true, "").Return(newRemoteSubscription, nil).Times(1)

		th.p.monitor.checkGlobalChatsSubscription(existingRemoteSubscription)
		expectLocalSubscription(th, t, newRemoteSubscription)
	})

	t.Run("local subscription exists, mismatched remote subscription", func(t *testing.T) {
		th.Reset(t)

		setupLocalSubscription(th, t)

		existingRemoteSubscription := &clientmodels.Subscription{
			ID:              model.NewId(),
			Type:            "allChats",
			ExpiresOn:       time.Now().Add(30 * time.Minute),
			NotificationURL: "http://example.com/plugins/com.mattermost.msteams-sync/",
		}

		th.appClientMock.On("DeleteSubscription", existingRemoteSubscription.ID).Return(nil).Times(1)

		newRemoteSubscription := &clientmodels.Subscription{
			ID:              model.NewId(),
			Type:            "allChats",
			ExpiresOn:       time.Now().Add(30 * time.Minute),
			NotificationURL: "http://example.com/plugins/com.mattermost.msteams-sync/",
		}

		th.appClientMock.On("SubscribeToChats", "http://example.com/plugins/com.mattermost.msteams-sync/", "webhooksecret", true, "").Return(newRemoteSubscription, nil).Times(1)

		th.p.monitor.checkGlobalChatsSubscription(existingRemoteSubscription)
		expectLocalSubscription(th, t, newRemoteSubscription)
	})

	t.Run("local subscription exists, matches remotes, should refresh", func(t *testing.T) {
		th.Reset(t)

		existingLocalSubscription := setupExpiringLocalSubscription(th, t)

		existingRemoteSubscription := &clientmodels.Subscription{
			ID:              existingLocalSubscription.SubscriptionID,
			Type:            existingLocalSubscription.Type,
			ExpiresOn:       existingLocalSubscription.ExpiresOn,
			NotificationURL: "http://example.com/plugins/com.mattermost.msteams-sync/",
		}

		newExpiryTime := time.Now().Add(30 * time.Minute)
		th.appClientMock.On("RefreshSubscription", existingRemoteSubscription.ID).Return(&newExpiryTime, nil)

		th.p.monitor.checkGlobalChatsSubscription(existingRemoteSubscription)

		existingRemoteSubscription.ExpiresOn = newExpiryTime
		expectLocalSubscription(th, t, existingRemoteSubscription)
	})

	t.Run("local subscription exists, matches remotes, fails to refresh but recreates successfully", func(t *testing.T) {
		th.Reset(t)

		existingLocalSubscription := setupExpiringLocalSubscription(th, t)

		existingRemoteSubscription := &clientmodels.Subscription{
			ID:              existingLocalSubscription.SubscriptionID,
			Type:            existingLocalSubscription.Type,
			ExpiresOn:       existingLocalSubscription.ExpiresOn,
			NotificationURL: "http://example.com/plugins/com.mattermost.msteams-sync/",
		}

		th.appClientMock.On("RefreshSubscription", existingRemoteSubscription.ID).Return(nil, fmt.Errorf("failed to refresh"))
		th.appClientMock.On("DeleteSubscription", existingRemoteSubscription.ID).Return(nil).Times(1)

		newRemoteSubscription := &clientmodels.Subscription{
			ID:              model.NewId(),
			Type:            "allChats",
			ExpiresOn:       time.Now().Add(30 * time.Minute),
			NotificationURL: "http://example.com/plugins/com.mattermost.msteams-sync/",
		}
		th.appClientMock.On("SubscribeToChats", "http://example.com/plugins/com.mattermost.msteams-sync/", "webhooksecret", true, "").Return(newRemoteSubscription, nil)

		th.p.monitor.checkGlobalChatsSubscription(existingRemoteSubscription)
		expectLocalSubscription(th, t, newRemoteSubscription)
	})

	t.Run("local subscription exists, matches remotes, fails to refresh and recreate", func(t *testing.T) {
		th.Reset(t)

		existingLocalSubscription := setupExpiringLocalSubscription(th, t)

		existingRemoteSubscription := &clientmodels.Subscription{
			ID:              existingLocalSubscription.SubscriptionID,
			Type:            existingLocalSubscription.Type,
			ExpiresOn:       existingLocalSubscription.ExpiresOn,
			NotificationURL: "http://example.com/plugins/com.mattermost.msteams-sync/",
		}

		th.appClientMock.On("RefreshSubscription", existingRemoteSubscription.ID).Return(nil, fmt.Errorf("failed to refresh"))
		th.appClientMock.On("DeleteSubscription", existingRemoteSubscription.ID).Return(nil).Times(1)

		th.appClientMock.On("SubscribeToChats", "http://example.com/plugins/com.mattermost.msteams-sync/", "webhooksecret", true, "").Return(nil, fmt.Errorf("failed to create"))

		th.p.monitor.checkGlobalChatsSubscription(existingRemoteSubscription)
		expectLocalSubscription(th, t, nil)
	})
}
