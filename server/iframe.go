// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	_ "embed"
	"net/http"
	"net/url"
	"strings"
)

// iFrame returns the iFrame HTML needed to host Mattermost within a MS Teams app.
func (a *API) iFrame(w http.ResponseWriter, _ *http.Request) {
	config := a.p.API.GetConfig()
	siteURL := *config.ServiceSettings.SiteURL
	if siteURL == "" {
		a.p.API.LogError("SiteURL cannot be empty for MS Teams iFrame")
		http.Error(w, "SiteURL is empty", http.StatusInternalServerError)
		return
	}

	parsedURL, err := url.Parse(siteURL)
	if err != nil {
		a.p.API.LogError("Invalid SiteURL for MS Teams iFrame", "error", err.Error())
		http.Error(w, "Invalid SiteURL", http.StatusInternalServerError)
		return
	}

	// Get the origin (scheme + host) for CSP
	origin := parsedURL.Scheme + "://" + parsedURL.Host

	// Set a minimal CSP for the wrapper page
	cspDirectives := []string{
		"default-src 'none'",        // Block all resources by default
		"frame-src " + origin,       // Only allow iframe to load content from Mattermost origin
		"style-src 'unsafe-inline'", // Allow inline styles for the iframe positioning
	}
	w.Header().Set("Content-Security-Policy", strings.Join(cspDirectives, "; "))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	html := strings.ReplaceAll(iFrameHTML, "{{SITE_URL}}", siteURL)

	w.Header().Set("Content-Type", "text/html")

	// set session cookie to indicate Mattermost is hosted in an iFrame, which allows
	// webapp to bypass "Where do you want to view this" page and set SameSite=none.
	http.SetCookie(w, &http.Cookie{
		Name:     "MMEMBED",
		Value:    "1",
		Path:     "/",
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})

	if _, err := w.Write([]byte(html)); err != nil {
		a.p.API.LogWarn("Unable to serve the iFrame", "error", err.Error())
	}
}

var iFrameHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Mattermost</title>
  <meta name="viewport" content="width=device-width, height=device-height, initial-scale=1.0">
</head>
<body>
	<iframe
		style="position:absolute;top:0px;width:100%;height:100vh;"
		src="{{SITE_URL}}" title="Mattermost">
	</iframe>
</body>
</html>`
