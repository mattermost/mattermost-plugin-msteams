// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessActivity(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "changes")

	sendRequest := func(t *testing.T, activities []msteams.Activity) (*http.Response, string) {
		t.Helper()

		data, err := json.Marshal(Activities{Value: activities})
		require.NoError(t, err)

		response, err := http.Post(apiURL, "text/json", bytes.NewReader(data))
		require.NoError(t, err)

		bodyBytes, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		bodyString := string(bodyBytes)

		return response, bodyString
	}

	t.Run("validation token", func(t *testing.T) {
		th.Reset(t)

		response, err := http.Post(apiURL+"?validationToken=test", "text/plain", nil)
		require.NoError(t, err)

		bodyBytes, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		bodyString := string(bodyBytes)

		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "test", bodyString)
	})

	t.Run("nil body", func(t *testing.T) {
		th.Reset(t)

		response, err := http.Post(apiURL, "text/plain", nil)
		require.NoError(t, err)

		bodyBytes, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		bodyString := string(bodyBytes)

		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		assert.Equal(t, "unable to get the activities from the message\n", bodyString)
	})

	t.Run("invalid body", func(t *testing.T) {
		th.Reset(t)

		response, err := http.Post(apiURL, "text/plain", bytes.NewReader([]byte("{")))
		require.NoError(t, err)

		bodyBytes, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		bodyString := string(bodyBytes)

		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		assert.Equal(t, "unable to get the activities from the message\n", bodyString)
	})

	t.Run("invalid webhook secret", func(t *testing.T) {
		th.Reset(t)

		activities := []msteams.Activity{
			{
				Resource:                       "teams('team-id')/channels('channel-id')/messages('message-id')",
				ChangeType:                     "created",
				ClientState:                    "invalid",
				SubscriptionExpirationDateTime: time.Now().Add(10 * time.Minute),
			},
		}

		response, bodyString := sendRequest(t, activities)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		assert.Equal(t, "Invalid webhook secret\n", bodyString)
	})

	t.Run("valid message", func(t *testing.T) {
		th.Reset(t)

		activities := []msteams.Activity{
			{
				Resource:                       "teams('team-id')/channels('channel-id')/messages('message-id')",
				ChangeType:                     "created",
				ClientState:                    "webhooksecret",
				SubscriptionExpirationDateTime: time.Now().Add(10 * time.Minute),
			},
		}

		response, bodyString := sendRequest(t, activities)
		assert.Equal(t, http.StatusAccepted, response.StatusCode)
		assert.Empty(t, bodyString)
	})

	t.Run("valid reply", func(t *testing.T) {
		th.Reset(t)

		activities := []msteams.Activity{
			{
				Resource:                       "teams('team-id')/channels('channel-id')/messages('message-id')/replies('reply-id')",
				ChangeType:                     "created",
				ClientState:                    "webhooksecret",
				SubscriptionExpirationDateTime: time.Now().Add(10 * time.Minute),
			},
		}

		response, bodyString := sendRequest(t, activities)
		assert.Equal(t, http.StatusAccepted, response.StatusCode)
		assert.Empty(t, bodyString)
	})

	t.Run("other resource", func(t *testing.T) {
		th.Reset(t)

		activities := []msteams.Activity{
			{
				Resource:                       "test",
				ChangeType:                     "created",
				ClientState:                    "webhooksecret",
				SubscriptionExpirationDateTime: time.Now().Add(10 * time.Minute),
			},
		}

		response, bodyString := sendRequest(t, activities)
		assert.Equal(t, http.StatusAccepted, response.StatusCode)
		assert.Empty(t, bodyString)
	})
}

func TestProcessLifecycle(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "lifecycle")

	sendRequest := func(t *testing.T, activities []msteams.Activity) (*http.Response, string) {
		t.Helper()

		data, err := json.Marshal(Activities{Value: activities})
		require.NoError(t, err)

		response, err := http.Post(apiURL, "text/json", bytes.NewReader(data))
		require.NoError(t, err)

		bodyBytes, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		bodyString := string(bodyBytes)

		return response, bodyString
	}

	t.Run("validation token", func(t *testing.T) {
		th.Reset(t)

		response, err := http.Post(apiURL+"?validationToken=test", "text/plain", nil)
		require.NoError(t, err)

		bodyBytes, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		bodyString := string(bodyBytes)

		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "test", bodyString)
	})

	t.Run("nil body", func(t *testing.T) {
		th.Reset(t)

		response, err := http.Post(apiURL, "text/plain", nil)
		require.NoError(t, err)

		bodyBytes, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		bodyString := string(bodyBytes)

		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		assert.Equal(t, "unable to get the lifecycle events from the message\n", bodyString)
	})

	t.Run("invalid body", func(t *testing.T) {
		th.Reset(t)

		response, err := http.Post(apiURL, "text/plain", bytes.NewReader([]byte("{")))
		require.NoError(t, err)

		bodyBytes, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		bodyString := string(bodyBytes)

		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		assert.Equal(t, "unable to get the lifecycle events from the message\n", bodyString)
	})

	t.Run("invalid webhook secret", func(t *testing.T) {
		th.Reset(t)

		activities := []msteams.Activity{
			{
				Resource:       "mockResource",
				ChangeType:     "mockChangeType",
				ClientState:    "mockClientState",
				LifecycleEvent: "reauthorizationRequired",
			},
		}

		response, bodyString := sendRequest(t, activities)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		assert.Equal(t, "Invalid webhook secret\n", bodyString)
	})

	t.Run("valid payload, unknown subscription", func(t *testing.T) {
		th.Reset(t)

		activities := []msteams.Activity{
			{
				SubscriptionID: model.NewId(),
				Resource:       "mockResource",
				ClientState:    "webhooksecret",
				ChangeType:     "mockChangeType",
				LifecycleEvent: "reauthorizationRequired",
			},
		}

		response, bodyString := sendRequest(t, activities)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Empty(t, bodyString)

		assert.Eventually(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_lifecycle_events_total",
				withLabel("event_type", "reauthorizationRequired"),
				withLabel("discarded_reason", metrics.DiscardedReasonUnusedSubscription),
			) == 1
		}, 5*time.Second, 500*time.Millisecond)
		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_subscriptions_total",
				withLabel("action", metrics.SubscriptionRefreshed),
			) > 0
		}, 5*time.Second, 500*time.Millisecond)
	})

	t.Run("valid payload, unknown event", func(t *testing.T) {
		th.Reset(t)

		subscription := storemodels.GlobalSubscription{
			SubscriptionID: model.NewId(),
			Type:           "allChats",
			ExpiresOn:      time.Now().Add(10 * time.Minute),
			Secret:         th.p.getConfiguration().WebhookSecret,
		}
		err := th.p.GetStore().SaveGlobalSubscription(subscription)
		require.NoError(t, err)

		activities := []msteams.Activity{
			{
				SubscriptionID: subscription.SubscriptionID,
				Resource:       "mockResource",
				ClientState:    "webhooksecret",
				ChangeType:     "mockChangeType",
				LifecycleEvent: "unknownEvent",
			},
		}

		response, bodyString := sendRequest(t, activities)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Empty(t, bodyString)

		assert.Eventually(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_lifecycle_events_total",
				withLabel("event_type", "unknownEvent"),
				withLabel("discarded_reason", metrics.DiscardedReasonUnknownLifecycleEvent),
			) == 1
		}, 5*time.Second, 500*time.Millisecond)
		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_subscriptions_total",
				withLabel("action", metrics.SubscriptionRefreshed),
			) > 0
		}, 5*time.Second, 500*time.Millisecond)
	})

	t.Run("valid event, refresh needed", func(t *testing.T) {
		th.Reset(t)

		subscription := storemodels.GlobalSubscription{
			SubscriptionID: model.NewId(),
			Type:           "allChats",
			ExpiresOn:      time.Now().Add(10 * time.Minute),
			Secret:         th.p.getConfiguration().WebhookSecret,
		}
		err := th.p.GetStore().SaveGlobalSubscription(subscription)
		require.NoError(t, err)

		activities := []msteams.Activity{
			{
				SubscriptionID: subscription.SubscriptionID,
				Resource:       "mockResource",
				ClientState:    "webhooksecret",
				ChangeType:     "mockChangeType",
				LifecycleEvent: "reauthorizationRequired",
			},
		}

		expiresOn := time.Now().Add(1 * time.Hour)
		th.appClientMock.On("RefreshSubscription", subscription.SubscriptionID).Return(&expiresOn, nil).Times(1)

		response, bodyString := sendRequest(t, activities)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Empty(t, bodyString)

		assert.Eventually(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_lifecycle_events_total",
				withLabel("event_type", "reauthorizationRequired"),
				withLabel("discarded_reason", metrics.DiscardedReasonNone),
			) == 1
		}, 5*time.Second, 500*time.Millisecond)
		assert.Eventually(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_subscriptions_total",
				withLabel("action", metrics.SubscriptionRefreshed),
			) == 1
		}, 5*time.Second, 500*time.Millisecond)
	})
}

func TestAutocompleteTeams(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "/autocomplete/teams")
	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	client1 := th.SetupClient(t, user1.Id)

	sendRequest := func(t *testing.T, user *model.User) (*http.Response, []model.AutocompleteListItem) {
		t.Helper()

		request, err := http.NewRequest(http.MethodGet, apiURL, nil)
		require.NoError(t, err)

		request.Header.Set(model.HeaderAuth, client1.AuthType+" "+client1.AuthToken)

		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, response.Body.Close())
		})

		var list []model.AutocompleteListItem
		if response.StatusCode == http.StatusOK {
			err := json.NewDecoder(response.Body).Decode(&list)
			require.Nil(t, err)
		}

		return response, list
	}

	t.Run("no client for user", func(t *testing.T) {
		th.Reset(t)

		response, list := sendRequest(t, user1)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Empty(t, list)
	})

	t.Run("failed to get teams list", func(t *testing.T) {
		th.Reset(t)

		th.ConnectUser(t, user1.Id)
		th.clientMock.On("ListTeams").Return(nil, errors.New("unable to get the teams list")).Times(1)

		response, list := sendRequest(t, user1)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Empty(t, list)
	})

	t.Run("single team returned", func(t *testing.T) {
		th.Reset(t)

		th.ConnectUser(t, user1.Id)
		th.clientMock.On("ListTeams").Return([]clientmodels.Team{
			{
				ID:          "mockTeamsTeamID-1",
				DisplayName: "mockDisplayName-1",
				Description: "mockDescription-1",
			},
		}, nil).Times(1)

		response, list := sendRequest(t, user1)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, []model.AutocompleteListItem{
			{
				Item:     "mockTeamsTeamID-1",
				Hint:     "mockDisplayName-1",
				HelpText: "mockDescription-1",
			},
		}, list)
	})

	t.Run("multiple teams returned", func(t *testing.T) {
		th.Reset(t)

		th.ConnectUser(t, user1.Id)
		th.clientMock.On("ListTeams").Return([]clientmodels.Team{
			{
				ID:          "mockTeamsTeamID-1",
				DisplayName: "mockDisplayName-1",
				Description: "mockDescription-1",
			},
			{
				ID:          "mockTeamsTeamID-2",
				DisplayName: "mockDisplayName-2",
				Description: "mockDescription-2",
			},
		}, nil).Times(1)

		response, list := sendRequest(t, user1)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, []model.AutocompleteListItem{
			{
				Item:     "mockTeamsTeamID-1",
				Hint:     "mockDisplayName-1",
				HelpText: "mockDescription-1",
			},
			{
				Item:     "mockTeamsTeamID-2",
				Hint:     "mockDisplayName-2",
				HelpText: "mockDescription-2",
			},
		}, list)
	})
}

func TestAutocompleteChannels(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "/autocomplete/channels")
	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	client1 := th.SetupClient(t, user1.Id)

	sendRequest := func(t *testing.T, user *model.User, queryParams string) (*http.Response, []model.AutocompleteListItem) {
		t.Helper()

		u := apiURL
		if queryParams != "" {
			u += "?" + url.Values{"parsed": {queryParams}}.Encode()
		}

		request, err := http.NewRequest(http.MethodGet, u, nil)
		require.NoError(t, err)

		request.Header.Set(model.HeaderAuth, client1.AuthType+" "+client1.AuthToken)

		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, response.Body.Close())
		})

		var list []model.AutocompleteListItem
		if response.StatusCode == http.StatusOK {
			err := json.NewDecoder(response.Body).Decode(&list)
			require.Nil(t, err)
		}

		return response, list
	}

	t.Run("no query parameters", func(t *testing.T) {
		th.Reset(t)

		response, list := sendRequest(t, user1, "")
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Empty(t, list)
	})

	t.Run("no client for user", func(t *testing.T) {
		th.Reset(t)

		response, list := sendRequest(t, user1, "mockData-1 mockData-2 mockData-3")
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Empty(t, list)
	})

	t.Run("failed to get channels list", func(t *testing.T) {
		th.Reset(t)

		th.ConnectUser(t, user1.Id)
		th.clientMock.On("ListChannels", "mockData-3").Return(nil, errors.New("unable to get the channels list")).Times(1)

		response, list := sendRequest(t, user1, "mockData-1 mockData-2 mockData-3")
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Empty(t, list)
	})

	t.Run("single channel returned", func(t *testing.T) {
		th.Reset(t)

		th.ConnectUser(t, user1.Id)
		th.clientMock.On("ListChannels", "mockData-3").Return([]clientmodels.Channel{
			{
				ID:          "mockTeamsChannelID-1",
				DisplayName: "mockDisplayName-1",
				Description: "mockDescription-1",
			},
		}, nil).Times(1)

		response, list := sendRequest(t, user1, "mockData-1 mockData-2 mockData-3")
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, []model.AutocompleteListItem{
			{
				Item:     "mockTeamsChannelID-1",
				Hint:     "mockDisplayName-1",
				HelpText: "mockDescription-1",
			},
		}, list)
	})

	t.Run("multiple channels returned", func(t *testing.T) {
		th.Reset(t)

		th.ConnectUser(t, user1.Id)
		th.clientMock.On("ListChannels", "mockData-3").Return([]clientmodels.Channel{
			{
				ID:          "mockTeamsChannelID-1",
				DisplayName: "mockDisplayName-1",
				Description: "mockDescription-1",
			},
			{
				ID:          "mockTeamsChannelID-2",
				DisplayName: "mockDisplayName-2",
				Description: "mockDescription-2",
			},
		}, nil).Times(1)

		response, list := sendRequest(t, user1, "mockData-1 mockData-2 mockData-3")
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, []model.AutocompleteListItem{
			{
				Item:     "mockTeamsChannelID-1",
				Hint:     "mockDisplayName-1",
				HelpText: "mockDescription-1",
			},
			{
				Item:     "mockTeamsChannelID-2",
				Hint:     "mockDisplayName-2",
				HelpText: "mockDescription-2",
			},
		}, list)
	})
}

func TestConnect(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "/connect")
	team := th.SetupTeam(t)

	sendRequest := func(t *testing.T, user *model.User, channelID, postID string) *http.Response {
		t.Helper()
		client1 := th.SetupClient(t, user.Id)

		u := apiURL

		values := make(url.Values)
		if channelID != "" {
			values["channel_id"] = []string{channelID}
		}
		if postID != "" {
			values["post_id"] = []string{postID}
		}
		if len(values) > 0 {
			u += "?" + values.Encode()
		}

		request, err := http.NewRequest(http.MethodGet, u, nil)
		require.NoError(t, err)

		request.Header.Set(model.HeaderAuth, client1.AuthType+" "+client1.AuthToken)
		client := &http.Client{
			// Don't follow redirects
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		response, err := client.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, response.Body.Close())
		})

		return response
	}

	t.Run("missing channel parameter", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		response := sendRequest(t, user1, "", "post_id")
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("missing post parameter", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		response := sendRequest(t, user1, "channel_id", "")
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("missing channel and post parameters", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		response := sendRequest(t, user1, "", "")
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("user already connected", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		response := sendRequest(t, user1, "channel_id", "post_id")
		assert.Equal(t, http.StatusForbidden, response.StatusCode)
	})

	t.Run("user connected", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		response := sendRequest(t, user1, "channel_id", "post_id")
		assert.Equal(t, http.StatusSeeOther, response.StatusCode)

		actualURL, err := url.Parse(response.Header.Get("Location"))
		require.NoError(t, err)
		assert.Equal(t, "login.microsoftonline.com", actualURL.Host)
		assert.Regexp(t, "oauth2/v2.0/authorize$", actualURL.Path)
	})
}

func TestOAuthRedirectHandler(t *testing.T) {
	t.Skip()
}

func TestGetConnectedUsers(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "/connected-users")
	team := th.SetupTeam(t)

	sendRequest := func(t *testing.T, user *model.User) (*http.Response, []storemodels.ConnectedUser) {
		t.Helper()
		client1 := th.SetupClient(t, user.Id)

		request, err := http.NewRequest(http.MethodGet, apiURL, nil)
		require.NoError(t, err)

		request.Header.Set(model.HeaderAuth, client1.AuthType+" "+client1.AuthToken)

		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, response.Body.Close())
		})

		var list []storemodels.ConnectedUser
		if response.StatusCode == http.StatusOK {
			err := json.NewDecoder(response.Body).Decode(&list)
			require.Nil(t, err)
		}

		return response, list
	}

	t.Run("insufficient permissions", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		response, connectedUsers := sendRequest(t, user)
		assert.Equal(t, http.StatusForbidden, response.StatusCode)
		assert.Empty(t, connectedUsers)
	})

	t.Run("no connected users", func(t *testing.T) {
		th.Reset(t)
		sysadmin := th.SetupSysadmin(t, team)

		response, connectedUsers := sendRequest(t, sysadmin)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Empty(t, connectedUsers)
	})

	t.Run("some connected users", func(t *testing.T) {
		th.Reset(t)
		sysadmin := th.SetupSysadmin(t, team)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)
		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)
		user3 := th.SetupUser(t, team)
		th.ConnectUser(t, user3.Id)
		user4 := th.SetupUser(t, team)
		th.ConnectUser(t, user4.Id)

		response, connectedUsers := sendRequest(t, sysadmin)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, []storemodels.ConnectedUser{
			{
				MattermostUserID: user1.Id,
				TeamsUserID:      "t" + user1.Id,
				FirstName:        user1.FirstName,
				LastName:         user1.LastName,
				Email:            user1.Email,
			},
			{
				MattermostUserID: user2.Id,
				TeamsUserID:      "t" + user2.Id,
				FirstName:        user2.FirstName,
				LastName:         user2.LastName,
				Email:            user2.Email,
			},
			{
				MattermostUserID: user3.Id,
				TeamsUserID:      "t" + user3.Id,
				FirstName:        user3.FirstName,
				LastName:         user3.LastName,
				Email:            user3.Email,
			},
			{
				MattermostUserID: user4.Id,
				TeamsUserID:      "t" + user4.Id,
				FirstName:        user4.FirstName,
				LastName:         user4.LastName,
				Email:            user4.Email,
			},
		}, connectedUsers)
	})
}

func TestGetConnectedUsersFile(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "/connected-users/download")
	team := th.SetupTeam(t)

	sendRequest := func(t *testing.T, user *model.User) (*http.Response, [][]string) {
		t.Helper()
		client1 := th.SetupClient(t, user.Id)

		request, err := http.NewRequest(http.MethodGet, apiURL, nil)
		require.NoError(t, err)

		request.Header.Set(model.HeaderAuth, client1.AuthType+" "+client1.AuthToken)

		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, response.Body.Close())
		})

		var records [][]string
		if response.StatusCode == http.StatusOK {
			assert.Equal(t, "text/csv", response.Header.Get("Content-Type"))
			assert.Equal(t, "attachment;filename=connected-users.csv", response.Header.Get("Content-Disposition"))

			csvReader := csv.NewReader(response.Body)
			records, err = csvReader.ReadAll()
			require.Nil(t, err)
		}

		return response, records
	}

	t.Run("insufficient permissions", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		response, connectedUsers := sendRequest(t, user)
		assert.Equal(t, http.StatusForbidden, response.StatusCode)
		assert.Empty(t, connectedUsers)
	})

	t.Run("no connected users", func(t *testing.T) {
		th.Reset(t)
		sysadmin := th.SetupSysadmin(t, team)

		response, connectedUsers := sendRequest(t, sysadmin)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, [][]string{
			{"First Name", "Last Name", "Email", "Mattermost User Id", "Teams User Id"},
		}, connectedUsers)
	})

	t.Run("some connected users", func(t *testing.T) {
		th.Reset(t)
		sysadmin := th.SetupSysadmin(t, team)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)
		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)
		user3 := th.SetupUser(t, team)
		th.ConnectUser(t, user3.Id)
		user4 := th.SetupUser(t, team)
		th.ConnectUser(t, user4.Id)

		response, connectedUsers := sendRequest(t, sysadmin)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, []string{
			"First Name", "Last Name", "Email", "Mattermost User Id", "Teams User Id",
		}, connectedUsers[0])
		assert.ElementsMatch(t, [][]string{
			{user1.FirstName, user1.LastName, user1.Email, user1.Id, "t" + user1.Id},
			{user2.FirstName, user2.LastName, user2.Email, user2.Id, "t" + user2.Id},
			{user3.FirstName, user3.LastName, user3.Email, user3.Id, "t" + user3.Id},
			{user4.FirstName, user4.LastName, user4.Email, user4.Id, "t" + user4.Id},
		}, connectedUsers[1:])
	})
}

func TestWhitelist(t *testing.T) {
	t.Skip()
}

func TestWhitelistDownload(t *testing.T) {
	t.Skip()
}

func TestNotifyConnect(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "/notify-connect")
	team := th.SetupTeam(t)

	sendRequest := func(t *testing.T, user *model.User) *http.Response {
		t.Helper()
		client1 := th.SetupClient(t, user.Id)

		u := apiURL

		request, err := http.NewRequest(http.MethodGet, u, nil)
		require.NoError(t, err)

		request.Header.Set(model.HeaderAuth, client1.AuthType+" "+client1.AuthToken)

		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, response.Body.Close())
		})

		return response
	}

	t.Run("not authorized", func(t *testing.T) {
		th.Reset(t)

		request, err := http.NewRequest(http.MethodGet, apiURL, nil)
		require.NoError(t, err)

		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		t.Cleanup(func() {
			require.NoError(t, response.Body.Close())
		})
	})

	t.Run("notify connect", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)

		response := sendRequest(t, user1)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})
}

func TestGetSiteStats(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "/stats/site")
	team := th.SetupTeam(t)

	sendRequest := func(t *testing.T, user *model.User) (*http.Response, string) {
		t.Helper()
		client1 := th.SetupClient(t, user.Id)

		request, err := http.NewRequest(http.MethodGet, apiURL, nil)
		require.NoError(t, err)

		request.Header.Set(model.HeaderAuth, client1.AuthType+" "+client1.AuthToken)

		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, response.Body.Close())
		})

		bodyBytes, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		bodyString := string(bodyBytes)

		return response, bodyString
	}

	t.Run("insufficient permissions", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		response, bodyString := sendRequest(t, user)
		assert.Equal(t, http.StatusForbidden, response.StatusCode)
		assert.Equal(t, "not able to authorize the user\n", bodyString)
	})

	t.Run("no connected users", func(t *testing.T) {
		th.Reset(t)
		sysadmin := th.SetupSysadmin(t, team)

		response, bodyString := sendRequest(t, sysadmin)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.JSONEq(t, `{"current_whitelist_users":0, "pending_invited_users":0, "total_connected_users":0, "total_active_users":0}`, bodyString)
	})

	t.Run("1 connected user", func(t *testing.T) {
		th.Reset(t)
		sysadmin := th.SetupSysadmin(t, team)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		err := th.p.store.SetUserLastChatReceivedAt(user1.Id, time.Now().Add(-4*24*time.Hour).UnixMicro())
		require.NoError(t, err)

		response, bodyString := sendRequest(t, sysadmin)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.JSONEq(t, `{"current_whitelist_users":0, "pending_invited_users":0, "total_connected_users":1,"total_active_users":1}`, bodyString)
	})

	t.Run("1 invited user, 2 whitelisted users", func(t *testing.T) {
		th.Reset(t)
		sysadmin := th.SetupSysadmin(t, team)

		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupUser(t, team)

		th.MarkUserWhitelisted(t, user1.Id)
		th.MarkUserWhitelisted(t, user2.Id)
		th.MarkUserInvited(t, user3.Id)

		response, bodyString := sendRequest(t, sysadmin)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.JSONEq(t, `{"current_whitelist_users":2, "pending_invited_users":1, "total_connected_users":0,"total_active_users":0}`, bodyString)
	})

	t.Run("10 connected users", func(t *testing.T) {
		th.Reset(t)
		sysadmin := th.SetupSysadmin(t, team)

		for i := 0; i < 10; i++ {
			user := th.SetupUser(t, team)
			th.ConnectUser(t, user.Id)

			if i < 5 {
				err := th.p.store.SetUserLastChatReceivedAt(user.Id, time.Now().Add(-4*24*time.Hour).UnixMicro())
				require.NoError(t, err)
			} else {
				err := th.p.store.SetUserLastChatReceivedAt(user.Id, time.Now().Add(-8*24*time.Hour).UnixMicro())
				require.NoError(t, err)
			}
		}

		response, bodyString := sendRequest(t, sysadmin)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.JSONEq(t, `{"current_whitelist_users":0, "pending_invited_users":0, "total_connected_users":10,"total_active_users":5}`, bodyString)
	})
}

func TestIFrameMattermostTab(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "/iframe/mattermostTab")
	team := th.SetupTeam(t)
	user := th.SetupUser(t, team)
	client := th.SetupClient(t, user.Id)

	th.Reset(t)

	request, err := http.NewRequest(http.MethodGet, apiURL, nil)
	require.NoError(t, err)

	request.Header.Set(model.HeaderAuth, client.AuthType+" "+client.AuthToken)

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, response.Body.Close())
	})

	bodyBytes, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	bodyString := string(bodyBytes)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Contains(t, bodyString, "<html")
	assert.Contains(t, bodyString, "</html>")

	// Verify iframe src matches site URL
	siteURL := th.p.API.GetConfig().ServiceSettings.SiteURL
	assert.Contains(t, bodyString, `src="`+*siteURL+`"`)

	// Verify security headers are set correctly
	require.NoError(t, err)
	expectedCSP := "style-src 'unsafe-inline'"
	assert.Equal(t, expectedCSP, response.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", response.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", response.Header.Get("Referrer-Policy"))

	// Verify MMEMBED cookie is set
	cookies := response.Cookies()
	var mmembedCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "MMEMBED" {
			mmembedCookie = cookie
			break
		}
	}
	require.NotNil(t, mmembedCookie, "MMEMBED cookie should be set")
	assert.Equal(t, "1", mmembedCookie.Value)
	// The cookie is not HttpOnly in the actual implementation
	assert.Equal(t, "/", mmembedCookie.Path)
	assert.True(t, mmembedCookie.Secure)
	assert.Equal(t, http.SameSiteNoneMode, mmembedCookie.SameSite)
}

func TestIFrameMattermostTabWithIdpURL(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "/iframe/mattermostTab")
	team := th.SetupTeam(t)
	user := th.SetupUser(t, team)
	client := th.SetupClient(t, user.Id)

	th.Reset(t)

	// Set IdpURL in config
	config := th.p.API.GetConfig()
	idpURL := "https://idp.example.com/saml"
	config.SamlSettings.IdpURL = &idpURL
	appErr := th.p.API.SaveConfig(config)
	require.Nil(t, appErr)

	request, err := http.NewRequest(http.MethodGet, apiURL, nil)
	require.NoError(t, err)

	request.Header.Set(model.HeaderAuth, client.AuthType+" "+client.AuthToken)

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, response.Body.Close())
	})

	bodyBytes, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	bodyString := string(bodyBytes)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Contains(t, bodyString, "<html")
	assert.Contains(t, bodyString, "</html>")

	// Verify iframe src matches site URL
	siteURL := th.p.API.GetConfig().ServiceSettings.SiteURL
	assert.Contains(t, bodyString, `src="`+*siteURL+`"`)

	// Verify security headers are set correctly with IdP URL included
	require.NoError(t, err)

	expectedCSP := "style-src 'unsafe-inline'"
	assert.Equal(t, expectedCSP, response.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", response.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", response.Header.Get("Referrer-Policy"))

	// Verify MMEMBED cookie is set
	cookies := response.Cookies()
	var mmembedCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "MMEMBED" {
			mmembedCookie = cookie
			break
		}
	}
	require.NotNil(t, mmembedCookie, "MMEMBED cookie should be set")
	assert.Equal(t, "1", mmembedCookie.Value)
	assert.Equal(t, "/", mmembedCookie.Path)
	assert.True(t, mmembedCookie.Secure)
	assert.Equal(t, http.SameSiteNoneMode, mmembedCookie.SameSite)
}

func TestConnectionStatus(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "/connection-status")
	team := th.SetupTeam(t)

	sendRequest := func(t *testing.T, user *model.User) (connected bool) {
		t.Helper()
		client := th.SetupClient(t, user.Id)

		request, err := http.NewRequest(http.MethodGet, apiURL, nil)
		require.NoError(t, err)

		request.Header.Set(model.HeaderAuth, client.AuthType+" "+client.AuthToken)

		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, response.Body.Close())
		})

		resMap := map[string]bool{}
		err = json.NewDecoder(response.Body).Decode(&resMap)
		require.NoError(t, err)

		return resMap["connected"]
	}

	t.Run("connected users should get true", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)
		th.ConnectUser(t, user.Id)

		connected := sendRequest(t, user)
		assert.True(t, connected)
	})

	t.Run("never connected users should get false", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		connected := sendRequest(t, user)
		assert.False(t, connected)
	})

	t.Run("disconnected users should get false", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)
		th.ConnectUser(t, user.Id)
		th.DisconnectUser(t, user.Id)

		connected := sendRequest(t, user)
		assert.False(t, connected)
	})
}
