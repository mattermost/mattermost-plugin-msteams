package main

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	PreferenceCategoryPlugin = "pp_" + pluginID
)

// getPrimaryPlatform returns the user's primary platform preference.
func (p *Plugin) getPrimaryPlatform(userID string) string {
	pref, appErr := p.API.GetPreferenceForUser(userID, PreferenceCategoryPlugin, storemodels.PreferenceNamePlatform)
	if appErr != nil {
		// Default to Mattermost if not found or an error occurred
		return storemodels.PreferenceValuePlatformMM
	}

	return pref.Value
}

// setPrimaryPlatform sets a user's primary platform preference.
func (p *Plugin) setPrimaryPlatform(userID string, primaryPlatform string) error {
	if primaryPlatform != storemodels.PreferenceValuePlatformMM && primaryPlatform != storemodels.PreferenceValuePlatformMSTeams {
		return fmt.Errorf("invalid primary platform: %s", primaryPlatform)
	}

	appErr := p.API.UpdatePreferencesForUser(userID, []model.Preference{{
		UserId:   userID,
		Category: PreferenceCategoryPlugin,
		Name:     storemodels.PreferenceNamePlatform,
		Value:    primaryPlatform,
	}})
	if appErr != nil {
		return appErr
	}

	return nil
}

func (p *Plugin) GetPreferenceCategoryName() string {
	return PreferenceCategoryPlugin
}
