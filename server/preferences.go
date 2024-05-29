package main

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	PreferenceCategoryPlugin = "pp_" + pluginID
)

func (p *Plugin) GetPreferenceCategoryName() string {
	return PreferenceCategoryPlugin
}

func (p *Plugin) getNotificationPreference(userID string) bool {
	// this call returns a generic error if the preference does not exist,
	// we can omit the error check here and return off by default
	pref, _ := p.API.GetPreferenceForUser(userID, PreferenceCategoryPlugin, storemodels.PreferenceNameNotification)
	return pref.Value == storemodels.PreferenceValueNotificationOn
}

func (p *Plugin) setNotificationPreference(userID string, enable bool) error {
	value := storemodels.PreferenceValueNotificationOff
	if enable {
		value = storemodels.PreferenceValueNotificationOn
	}

	appErr := p.updatePreferenceForUser(userID, storemodels.PreferenceNameNotification, value)
	if appErr != nil {
		return fmt.Errorf("failed to set notification status: %w", appErr)
	}

	return nil
}

func (p *Plugin) updatePreferenceForUser(userID string, name string, value string) *model.AppError {
	return p.API.UpdatePreferencesForUser(userID, []model.Preference{{
		UserId:   userID,
		Category: PreferenceCategoryPlugin,
		Name:     name,
		Value:    value,
	}})
}
