// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"io"
	"net/http"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	// Verify iframe src is correct
	assert.Contains(t, bodyString, `src="about:blank"`)

	// Verify the SITE_URL is present
	siteURL := th.p.API.GetConfig().ServiceSettings.SiteURL
	assert.Contains(t, bodyString, `iframe.src = '`+*siteURL+`'`)

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

	// Verify src has had site URL replaced
	siteURL := th.p.API.GetConfig().ServiceSettings.SiteURL
	assert.Contains(t, bodyString, `"`+*siteURL+`"`)
	assert.NotContains(t, bodyString, "SITE_URL")

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

func TestGetCookieDomain(t *testing.T) {
	tests := []struct {
		name                     string
		siteURL                  string
		allowCookiesForSubdomain bool
		expected                 string
	}{
		{
			name:                     "Allow cookies for subdomains with valid URL",
			siteURL:                  "https://example.mattermost.com",
			allowCookiesForSubdomain: true,
			expected:                 "example.mattermost.com",
		},
		{
			name:                     "Allow cookies for subdomains with invalid URL",
			siteURL:                  "invalid-url",
			allowCookiesForSubdomain: true,
			expected:                 "",
		},
		{
			name:                     "Disallow cookies for subdomains",
			siteURL:                  "https://example.mattermost.com",
			allowCookiesForSubdomain: false,
			expected:                 "",
		},
		{
			name:                     "Allow cookies for subdomains with URL containing port",
			siteURL:                  "https://example.mattermost.com:8065",
			allowCookiesForSubdomain: true,
			expected:                 "example.mattermost.com",
		},
		{
			name:                     "Allow cookies for subdomains with localhost",
			siteURL:                  "http://localhost:8065",
			allowCookiesForSubdomain: true,
			expected:                 "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &model.Config{}
			config.ServiceSettings.SiteURL = &tt.siteURL
			config.ServiceSettings.AllowCookiesForSubdomains = &tt.allowCookiesForSubdomain

			result := getCookieDomain(config)
			assert.Equal(t, tt.expected, result)
		})
	}
}
