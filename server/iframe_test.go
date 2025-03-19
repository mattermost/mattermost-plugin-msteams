// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"net/http"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIFrameAuthenticate(t *testing.T) {
	th := setupTestHelper(t)
	apiURL := th.pluginURL(t, "/iframe/authenticate")

	// Create a client that doesn't follow redirects
	httpClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	t.Run("already logged in user", func(t *testing.T) {
		th.Reset(t)

		team := th.SetupTeam(t)
		user := th.SetupUser(t, team)
		client := th.SetupClient(t, user.Id)

		request, err := http.NewRequest(http.MethodGet, apiURL, nil)
		require.NoError(t, err)

		// Set the Mattermost-User-ID header to simulate an already logged in user
		request.Header.Set("Mattermost-User-ID", user.Id)
		request.Header.Set(model.HeaderAuth, client.AuthType+" "+client.AuthToken)

		response, err := httpClient.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, response.Body.Close())
		})

		// Should redirect to home page
		assert.Equal(t, http.StatusSeeOther, response.StatusCode)
		assert.Equal(t, "/", response.Header.Get("Location"))
	})

	t.Run("missing token", func(t *testing.T) {
		th.Reset(t)

		request, err := http.NewRequest(http.MethodGet, apiURL, nil)
		require.NoError(t, err)

		response, err := httpClient.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, response.Body.Close())
		})

		// Should return an error
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})

	t.Run("invalid token", func(t *testing.T) {
		th.Reset(t)

		request, err := http.NewRequest(http.MethodGet, apiURL+"?token=invalid_token", nil)
		require.NoError(t, err)

		response, err := httpClient.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, response.Body.Close())
		})

		// Should return an error
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})

	// Note: Testing with a valid token would require mocking the JWT validation
	// and claims extraction, which would be more complex and require additional setup
}

func TestGetCookieDomain(t *testing.T) {
	tests := []struct {
		name                     string
		siteURL                  string
		allowCookiesForSubdomain *bool
		expected                 string
	}{
		{
			name:                     "Allow cookies for subdomains with valid URL",
			siteURL:                  "https://example.mattermost.com",
			allowCookiesForSubdomain: model.NewPointer(true),
			expected:                 "example.mattermost.com",
		},
		{
			name:                     "Allow cookies for subdomains with invalid URL",
			siteURL:                  "invalid-url",
			allowCookiesForSubdomain: model.NewPointer(true),
			expected:                 "",
		},
		{
			name:                     "Disallow cookies for subdomains",
			siteURL:                  "https://example.mattermost.com",
			allowCookiesForSubdomain: model.NewPointer(false),
			expected:                 "",
		},
		{
			name:                     "Allow cookies for subdomains with URL containing port",
			siteURL:                  "https://example.mattermost.com:8065",
			allowCookiesForSubdomain: model.NewPointer(true),
			expected:                 "example.mattermost.com",
		},
		{
			name:                     "Allow cookies for subdomains with localhost",
			siteURL:                  "http://localhost:8065",
			allowCookiesForSubdomain: model.NewPointer(true),
			expected:                 "localhost",
		},
		{
			name:                     "Nil AllowCookiesForSubdomain",
			siteURL:                  "https://example.mattermost.com",
			allowCookiesForSubdomain: nil,
			expected:                 "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &model.Config{}
			config.ServiceSettings.SiteURL = &tt.siteURL
			config.ServiceSettings.AllowCookiesForSubdomains = tt.allowCookiesForSubdomain

			result := getCookieDomain(config)
			assert.Equal(t, tt.expected, result)
		})
	}
}
