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

func (p *Plugin) getNotificationPreference(userID string) bool {
	// this call returns a generic error if the preference does not exist,
	// we can omit the error check here and let the code assume it's on turned on
	pref, _ := p.API.GetPreferenceForUser(userID, PreferenceCategoryPlugin, storemodels.PreferenceNameNotification)
	return pref.Value == storemodels.PreferenceValueNotificationOn
}

func (p *Plugin) setNotificationPreference(userID string, enable bool) error {
	value := storemodels.PreferenceValueNotificationOff
	if enable {
		value = storemodels.PreferenceValueNotificationOn
	}

	err := p.API.UpdatePreferencesForUser(userID, []model.Preference{{
		UserId:   userID,
		Category: PreferenceCategoryPlugin,
		Name:     storemodels.PreferenceNameNotification,
		Value:    value,
	}})
	if err != nil {
		return fmt.Errorf("failed to set notification status: %w", err)
	}

	return nil
}
