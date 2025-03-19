// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/v8/channels/utils"
	"github.com/sirupsen/logrus"
)

//go:embed iframe.html
var iFrameHTML string

// iFrame returns the iFrame HTML needed to host Mattermost within a MS Teams app.
func (a *API) iFrame(w http.ResponseWriter, _ *http.Request) {
	// Set a minimal CSP for the wrapper page
	cspDirectives := []string{
		"style-src 'unsafe-inline'", // Allow inline styles for the iframe positioning
	}
	w.Header().Set("Content-Security-Policy", strings.Join(cspDirectives, "; "))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	html, err := a.formatTemplate(iFrameHTML)
	if err != nil {
		a.p.API.LogError("Failed to format iFrame HTML", "error", err.Error())
		http.Error(w, "Failed to format iFrame HTML", http.StatusInternalServerError)
		return
	}

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

// formatTemplate formats the iFrame HTML template with the site URL and plugin ID
func (a *API) formatTemplate(template string) (string, error) {
	config := a.p.API.GetConfig()
	siteURL := *config.ServiceSettings.SiteURL
	if siteURL == "" {
		return "", fmt.Errorf("ServiceSettings.SiteURL cannot be empty for MS Teams iFrame")
	}

	html := strings.ReplaceAll(template, "{{SITE_URL}}", siteURL)
	html = strings.ReplaceAll(html, "{{PLUGIN_ID}}", url.PathEscape(manifest.Id))
	html = strings.ReplaceAll(html, "{{TENANT_ID}}", a.p.getConfiguration().TenantID)
	return html, nil
}

// authenticate expects a Microsoft Entra ID in the Authorization header, and uses that
// to authenticate to the corresponding user in Mattermost, if it exists.
func (a *API) authenticate(w http.ResponseWriter, r *http.Request) {
	var logger logrus.FieldLogger
	logger = logrus.StandardLogger()

	redirectPath := "/"

	// Check if we have a subEntityID coming from the Microsoft Teams SDK to redirect the user to the correct URL.
	// We use this from the Team's notifications to redirect the user to what triggered the notification, in this case,
	// a post.
	subEntityID := r.URL.Query().Get("sub_entity_id")
	if subEntityID != "" {
		if strings.HasPrefix(subEntityID, "post_") {
			postID := strings.TrimPrefix(subEntityID, "post_")
			post, err := a.p.API.GetPost(postID)
			if err != nil {
				logger.WithError(err).Error("Failed to get post to generate redirect path from subEntityId")
			}

			channel, appErr := a.p.API.GetChannel(post.ChannelId)
			if appErr != nil {
				logger.WithError(appErr).Error("Failed to get channel to generate redirect path from subEntityId")
			}

			team, appErr := a.p.API.GetTeam(channel.TeamId)
			if appErr != nil {
				logger.WithError(appErr).Error("Failed to get team to generate redirect path from subEntityId")
			}

			redirectPath = fmt.Sprintf("/%s/pl/%s", team.Name, postID)
		}
	}

	// If the user is already logged in, redirect to the home page.
	// TODO: Refactor the user properties setup to a function and call it from here if the user is already logged in
	// just in case the user logs in from a tabApp in a browser.
	if r.Header.Get("Mattermost-User-ID") != "" {
		logger = logger.WithField("user_id", r.Header.Get("Mattermost-User-ID"))
		logger.Info("Skipping authentication, user already logged in")
		http.Redirect(w, r, redirectPath, http.StatusSeeOther)
		return
	}

	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		logger.Error("App ID was not sent with the authentication request")
	}

	config := a.p.apiClient.Configuration.GetConfig()

	enableDeveloper := config.ServiceSettings.EnableDeveloper

	// Ideally we'd accept the token via an Authorization header, but for now get it from the query string.
	// token := r.Header.Get("Authorization")
	token := r.URL.Query().Get("token")

	// Validate the token in the request, handling all errors if invalid.
	expectedTenantIDs := []string{a.p.getConfiguration().TenantID}
	claims, validationErr := validateToken(a.p.tabAppJWTKeyFunc, token, expectedTenantIDs, enableDeveloper != nil && *enableDeveloper)
	if validationErr != nil {
		handleErrorWithCode(logger, w, validationErr.StatusCode, validationErr.Message, validationErr.Err)
		return
	}

	if claims == nil {
		handleErrorWithCode(logger, w, http.StatusUnauthorized, "Invalid token claims", nil)
		return
	}

	oid, ok := claims["oid"].(string)
	if !ok || oid == "" {
		logger.Error("Missing or empty claim for oid")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	logger = logger.WithField("oid", oid)

	ssoUsername, ok := claims["unique_name"].(string)
	if !ok || ssoUsername == "" {
		logger.Warn("Missing or empty claim for unique_name")

		ssoUsername, ok = claims["preferred_username"].(string)
		if !ok || ssoUsername == "" {
			logger.Error("Missing or empty claim for unique_name or preferred_username")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
	}

	mmUser, err := a.p.apiClient.User.GetByEmail(ssoUsername)
	if err != nil && err != pluginapi.ErrNotFound {
		logger.WithError(err).Error("Failed to query Mattermost user matching unique_name")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	} else if mmUser == nil {
		logger.Warn("No Mattermost user matching unique_name, redirecting to login")

		// Redirect to the home page
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	logger = logger.WithField("user_id", mmUser.Id)

	if mmUser.DeleteAt != 0 {
		logger.Warn("Mattermost user is archived, redirecting to login")

		// Redirect to the home page
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Keep track of the unique_name and oid in the user's properties to support
	// notifications in the future.
	mmUser.Props[getUserPropKey("sso_username")] = ssoUsername
	mmUser.Props[getUserPropKey("oid")] = oid
	if appID != "" {
		mmUser.Props[getUserPropKey("app_id")] = appID
	}

	// Update the user with the claims
	err = a.p.apiClient.User.Update(mmUser)
	if err != nil {
		logger.WithError(err).Error("Failed to update Mattermost user with claims")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// This is effectively copied from https://github.com/mattermost/mattermost/blob/a184e5677d28433495b0cde764bfd99700838740/server/channels/app/login.go#L287
	secure := true
	maxAgeSeconds := *config.ServiceSettings.SessionLengthWebInHours * 60 * 60
	domain := getCookieDomain(config)
	subpath, _ := utils.GetSubpathFromConfig(config)

	jwtExpiresAt, err := claims.GetExpirationTime()
	if err != nil || jwtExpiresAt == nil {
		logger.WithError(err).Error("Missing or invalid expiration time claim")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	expiresAt := jwtExpiresAt.Time

	session, err := a.p.apiClient.Session.Create(&model.Session{
		UserId:    mmUser.Id,
		ExpiresAt: model.GetMillisForTime(expiresAt),
	})
	if err != nil {
		logger.WithError(err).Error("Failed to create session for Mattermost user")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	sessionCookie := &http.Cookie{
		Name:     model.SessionCookieToken,
		Value:    session.Token,
		Path:     subpath,
		MaxAge:   maxAgeSeconds,
		Expires:  expiresAt,
		HttpOnly: true,
		Domain:   domain,
		Secure:   secure,
		SameSite: http.SameSiteNoneMode,
	}

	userCookie := &http.Cookie{
		Name:     model.SessionCookieUser,
		Value:    mmUser.Id,
		Path:     subpath,
		MaxAge:   maxAgeSeconds,
		Expires:  expiresAt,
		Domain:   domain,
		Secure:   secure,
		SameSite: http.SameSiteNoneMode,
	}

	csrfCookie := &http.Cookie{
		Name:    model.SessionCookieCsrf,
		Value:   session.GetCSRF(),
		Path:    subpath,
		MaxAge:  maxAgeSeconds,
		Expires: expiresAt,
		Domain:  domain,
		Secure:  secure,
	}

	http.SetCookie(w, sessionCookie)
	http.SetCookie(w, userCookie)
	http.SetCookie(w, csrfCookie)

	// Redirect to the home page
	http.Redirect(w, r, redirectPath, http.StatusSeeOther)
}

// MessageHasBeenPosted is called when a message is posted in Mattermost. We rely on it to send a user activity notification
// to Microsoft Teams when a user is mentioned in a message.
// This is called in a controller Goroutine in the server side so there's no need to worry about concurrency here.
func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	p.API.LogError("Message has been posted", "post_id", post.Id, "post_participants", post.Participants)

	context := map[string]string{
		"subEntityId": fmt.Sprintf("post_%s", post.Id),
	}

	jsonContext, err := json.Marshal(context)
	if err != nil {
		p.API.LogError("Failed to marshal context", "error", err.Error())
		return
	}

	urlParams := url.Values{}
	urlParams.Set("context", string(jsonContext))

	for _, mention := range extractMentionsFromPost(post) {
		u, err := p.apiClient.User.GetByUsername(mention)
		if err != nil {
			p.API.LogError("Failed to get user", "error", err.Error())
			continue
		}

		msteamsUserID, exists := u.GetProp(getUserPropKey("user_id"))
		if !exists {
			p.API.LogError("MSTeams user ID is empty. Not sending notification.")
			continue
		}

		appID, exists := u.GetProp(getUserPropKey("app_id"))
		if !exists {
			p.API.LogError("MSTeams app ID is empty. Not sending notification.")
			continue
		}

		postAuthor, err := p.apiClient.User.Get(post.UserId)
		if err != nil {
			p.API.LogError("Failed to get post author", "error", err.Error())
			continue
		}

		// Sending the post author requires the proper variable to be set in the manifest:
		// "activities": {
		// 	"activityTypes": [
		// 	  {
		// 		"type": "mattermost_mention_with_name",
		// 		"description": "New message in Mattermost for the Teams user",
		// 		"templateText": "{post_author} mentioned you in Mattermost."
		// 	  }
		// 	]
		// }
		if err := p.msteamsAppClient.SendUserActivity(msteamsUserID, "mattermost_mention_with_name", post.Message, url.URL{
			Scheme:   "https",
			Host:     "teams.microsoft.com",
			Path:     "/l/entity/" + appID + "/" + context["subEntityId"],
			RawQuery: urlParams.Encode(),
		}, map[string]string{
			"post_author": postAuthor.GetDisplayName(model.ShowNicknameFullName),
		}); err != nil {
			p.API.LogError("Failed to send user activity notification", "error", err.Error())
		}
	}
}

func extractMentionsFromPost(post *model.Post) []string {
	// Regular expression to find mentions of the form @username
	mentionRegex := regexp.MustCompile(`@[a-zA-Z0-9._-]+`)
	matches := mentionRegex.FindAllString(post.Message, -1)

	// Remove the '@' symbol from each mention
	mentions := []string{}
	for _, match := range matches {
		mentions = append(mentions, match[1:]) // Remove the '@'
	}
	return mentions
}

func getCookieDomain(config *model.Config) string {
	if config.ServiceSettings.AllowCookiesForSubdomains != nil && *config.ServiceSettings.AllowCookiesForSubdomains && config.ServiceSettings.SiteURL != nil {
		if siteURL, err := url.Parse(*config.ServiceSettings.SiteURL); err == nil {
			return siteURL.Hostname()
		}
	}
	return ""
}

func getUserPropKey(key string) string {
	return "com.mattermost.plugin-msteams-devsecops." + key
}
