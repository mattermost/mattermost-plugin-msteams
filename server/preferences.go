package main

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	PreferenceCategoryPlugin = "pp_" + pluginID

	// The preference with this name stores the user's choice of primary platform.
	PreferenceNamePlatform         = "platform"
	PreferenceValuePlatformMM      = "mattermost"
	PreferenceValuePlatformMSTeams = "msteams"
)

// getPrimaryPlatform returns the user's primary platform preference.
func (p *Plugin) getPrimaryPlatform(userID string) string {
	pref, appErr := p.API.GetPreferenceForUser(userID, PreferenceCategoryPlugin, PreferenceNamePlatform)
	if appErr != nil {
		// Default to Mattermost if not found or an error occurred
		return PreferenceValuePlatformMM
	}

	return pref.Value
}

// setPrimaryPlatform sets a user's primary platform preference.
func (p *Plugin) setPrimaryPlatform(userID string, primaryPlatform string) error {
	if primaryPlatform != PreferenceValuePlatformMM && primaryPlatform != PreferenceValuePlatformMSTeams {
		return fmt.Errorf("invalid primary platform: %s", primaryPlatform)
	}

	appErr := p.API.UpdatePreferencesForUser(userID, []model.Preference{{
		UserId:   userID,
		Category: PreferenceCategoryPlugin,
		Name:     PreferenceNamePlatform,
		Value:    primaryPlatform,
	}})
	if appErr != nil {
		return appErr
	}

	return nil
}
