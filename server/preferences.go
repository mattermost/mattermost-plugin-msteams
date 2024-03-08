package main

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
