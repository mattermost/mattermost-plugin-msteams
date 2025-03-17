// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
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
	html = strings.ReplaceAll(html, "{{PLUGIN_ID}}", manifest.Id)
	html = strings.ReplaceAll(html, "{{TENANT_ID}}", a.p.getConfiguration().TenantID)
	return html, nil
}

// authenticate expects a Microsoft Entra ID in the Authorization header, and uses that
// to authenticate to the corresponding user in Mattermost, if it exists.
func (a *API) authenticate(w http.ResponseWriter, r *http.Request) {
	var logger logrus.FieldLogger
	logger = logrus.StandardLogger()

	// If the user is already logged in, redirect to the home page.
	// TODO: Refactor the user properties setup to a function and call it from here if the user is already logged in
	// just in case the user logs in from a tabApp in a browser.
	if r.Header.Get("Mattermost-User-ID") != "" {
		logger = logger.WithField("user_id", r.Header.Get("Mattermost-User-ID"))
		logger.Info("Skipping authentication, user already logged in")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
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

	oid, ok := claims["oid"].(string)
	if !ok {
		logger.Error("No claim for oid")
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	logger = logger.WithField("oid", oid)

	ssoUsername, ok := claims["unique_name"].(string)
	if !ok {
		logger.Warn("no unique_name claim")

		ssoUsername, ok = claims["preferred_username"].(string)
		if !ok {
			logger.Error("No claim for unique_name or preferred_username")
			http.Error(w, "", http.StatusBadRequest)
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
	mmUser.Props["com.mattermost.plugin-msteams-devsecops.sso_username"] = ssoUsername
	mmUser.Props["com.mattermost.plugin-msteams-devsecops.oid"] = oid

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

	expiresAt := time.Unix(model.GetMillis()/1000+int64(maxAgeSeconds), 0)

	session, err := a.p.apiClient.Session.Create(&model.Session{
		UserId: mmUser.Id,
		// TODO, should we allow this to be configurable?
		ExpiresAt: model.GetMillis() + (1000 * 60 * 60 * 24 * 1), // 1 day
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
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func getCookieDomain(config *model.Config) string {
	if *config.ServiceSettings.AllowCookiesForSubdomains {
		if siteURL, err := url.Parse(*config.ServiceSettings.SiteURL); err == nil {
			return siteURL.Hostname()
		}
	}
	return ""
}
