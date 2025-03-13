// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	_ "embed"
	"net/http"
	"strings"
)

// iFrame returns the iFrame HTML needed to host Mattermost within a MS Teams app.
func (a *API) iFrame(w http.ResponseWriter, _ *http.Request) {
	config := a.p.API.GetConfig()
	siteURL := *config.ServiceSettings.SiteURL
	if siteURL == "" {
		a.p.API.LogError("ServiceSettings.SiteURL cannot be empty for MS Teams iFrame")
		http.Error(w, "ServiceSettings.SiteURL is empty", http.StatusInternalServerError)
		return
	}

	// Set a minimal CSP for the wrapper page
	cspDirectives := []string{
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
  <title>Mattermost DevSecOps</title>
  <meta name="viewport" content="width=device-width, height=device-height, initial-scale=1.0">
  <script 
    src="https://res.cdn.office.net/teams-js/2.34.0/js/MicrosoftTeams.min.js"
    integrity="sha384-brW9AazbKR2dYw2DucGgWCCcmrm2oBFV4HQidyuyZRI/TnAkmOOnTARSTdps3Hwt"
    crossorigin="anonymous"
  ></script>
</head>
<body>
    <iframe
        style="position: absolute; top: 0; left: 0; width: 100%; height: 100%; border: none;"
        src="{{SITE_URL}}" title="Mattermost DevSecOps" onload="notifyAppLoaded()">
    </iframe>
  <script>
    // Initialize the Microsoft Teams SDK
    microsoftTeams.app.initialize(["{{SITE_URL}}"]);

    // Listen for a message from the iframe "mattermost_external_auth_login" sent by using window.parent.postMessage
    window.addEventListener('message', async function (event) {
      console.log('Received message from Mattermost:', event.data);

      if (event.data.type === 'mattermost_external_auth_login') {
        console.log("Authenticating...");

        microsoftTeams.authentication.authenticate({
          url: event.data.href,
          width: 600,
          height: 535,
          isExternal: false,
        })
        .then(microsoftTeams.authentication.getAuthToken)
        .then(microsoftTeams.app.getContext)
        .then(redirectToUserSetupPage)
        .catch(function (e) {
          console.error("Authentication failed:", e);
        });
      }
    });

    function notifyAppLoaded() {
      return microsoftTeams.app.notifySuccess();
    }

    function updateUserWithTeamsContext(context) {
      // Update current Mattermost user with the token
      return fetch("{{SITE_URL}}/plugins/com.mattermost.plugin-msteams-devsecops/users/setup", {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          user_id: context.user.id,
          user_principal_name: context.user.userPrincipalName,
        })      
      }).then(function (response) {
        console.log("User updated successfully");
        return response;
      }).catch(function (error) {
        return "Error updating user: " + error;
      });
    }

    // Redirect to the user setup page where we will store the user information and redirect back to the Mattermost login page
    function redirectToUserSetupPage(context) {
      iframe = document.querySelector('iframe');
      iframe.src = "{{SITE_URL}}/plugins/com.mattermost.plugin-msteams-devsecops/users/setup?id=" + context.user.id + "&user_principal_name=" + context.user.userPrincipalName + "&_t=" + new Date().getTime();
    }
  </script>
</body>
</html>`
