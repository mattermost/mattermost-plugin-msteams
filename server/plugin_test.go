// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestGetURL(t *testing.T) {
	testCases := []struct {
		Name     string
		URL      string
		Expected string
	}{
		{
			Name:     "no subpath, ending with /",
			URL:      "https://example.com/",
			Expected: "https://example.com/plugins/" + pluginID,
		},
		{
			Name:     "no subpath, not ending with /",
			URL:      "https://example.com",
			Expected: "https://example.com/plugins/" + pluginID,
		},
		{
			Name:     "with subpath, ending with /",
			URL:      "https://example.com/subpath/",
			Expected: "https://example.com/subpath/plugins/" + pluginID,
		},
		{
			Name:     "with subpath, not ending with /",
			URL:      "https://example.com/subpath",
			Expected: "https://example.com/subpath/plugins/" + pluginID,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			config := &model.Config{}
			config.SetDefaults()
			config.ServiceSettings.SiteURL = model.NewString(testCase.URL)

			actual := getURL(config)
			assert.Equal(t, testCase.Expected, actual)
		})
	}
}

func TestGetRelativeURL(t *testing.T) {
	testCases := []struct {
		Name     string
		URL      string
		Expected string
	}{
		{
			Name:     "Empty URL",
			URL:      "",
			Expected: "/plugins/" + pluginID,
		},
		{
			Name:     "no subpath, ending with /",
			URL:      "https://example.com/",
			Expected: "/plugins/" + pluginID,
		},
		{
			Name:     "no subpath, not ending with /",
			URL:      "https://example.com",
			Expected: "/plugins/" + pluginID,
		},
		{
			Name:     "with subpath, ending with /",
			URL:      "https://example.com/subpath/",
			Expected: "/subpath/plugins/" + pluginID,
		},
		{
			Name:     "with subpath, not ending with /",
			URL:      "https://example.com/subpath",
			Expected: "/subpath/plugins/" + pluginID,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			config := &model.Config{}
			config.SetDefaults()
			config.ServiceSettings.SiteURL = model.NewString(testCase.URL)

			actual := getRelativeURL(config)
			assert.Equal(t, testCase.Expected, actual)
		})
	}
}

func TestGetClientForUser(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("no such user", func(t *testing.T) {
		th.Reset(t)

		client, err := th.p.GetClientForUser("unknown")
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("user never connected", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)

		client, err := th.p.GetClientForUser(user.Id)
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("user disconnected", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		th.ConnectUser(t, user.Id)
		th.DisconnectUser(t, user.Id)

		client, err := th.p.GetClientForUser(user.Id)
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("user connected", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		th.ConnectUser(t, user.Id)

		client, err := th.p.GetClientForUser(user.Id)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})
}

func TestGetClientForTeamsUser(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("no such user", func(t *testing.T) {
		th.Reset(t)

		client, err := th.p.GetClientForTeamsUser("unknown")
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("user never connected", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)

		client, err := th.p.GetClientForTeamsUser("t" + user.Id)
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("user disconnected", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		th.ConnectUser(t, user.Id)
		th.DisconnectUser(t, user.Id)

		client, err := th.p.GetClientForTeamsUser("t" + user.Id)
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("user connected", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		th.ConnectUser(t, user.Id)

		client, err := th.p.GetClientForTeamsUser("t" + user.Id)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})
}

func TestSyncUsers(t *testing.T) {
	t.Skip("Not yet implemented")
}
