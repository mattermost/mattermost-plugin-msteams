package main

import (
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func (p *Plugin) PreferencesHaveChanged(c *plugin.Context, preferences []model.Preference) {
	p.updateAutomutingOnPreferencesChanged(c, preferences)
}

func (p *Plugin) updateAutomutingOnPreferencesChanged(_ *plugin.Context, preferences []model.Preference) {
	userIDsToEnable, userIDsToDisable := getUsersWhoChangedPlatform(preferences)

	for _, userID := range userIDsToEnable {
		if connected, err := p.IsUserConnected(userID); err != nil {
			p.API.LogWarn(
				"Unable to potentially enable automute for user",
				"user_id", userID,
				"error", err.Error(),
			)
		} else if !connected {
			continue
		}

		p.notifyUserTeamsPrimary(userID)

		if _, err := p.enableAutomute(userID); err != nil {
			p.API.LogWarn(
				"Unable to mute channels for a user who set their primary platform to Teams",
				"user_id", userID,
				"error", err.Error(),
			)
		}
	}

	for _, userID := range userIDsToDisable {
		if connected, err := p.IsUserConnected(userID); err != nil {
			p.API.LogWarn(
				"Unable to determine if user connected",
				"user_id", userID,
				"error", err.Error(),
			)
		} else if connected {
			// Don't notify if disconnected
			p.notifyUserMattermostPrimary(userID)
		}

		_, err := p.disableAutomute(userID)
		if err != nil {
			p.API.LogWarn(
				"Unable to unmute channels for a user who set their primary platform to Mattermost",
				"user_id", userID,
				"error", err.Error(),
			)
		}
	}
}

func getUsersWhoChangedPlatform(preferences []model.Preference) (usersWithTeamsPrimary []string, usersWithMMPrimary []string) {
	for _, preference := range preferences {
		if preference.Category == PreferenceCategoryPlugin && preference.Name == storemodels.PreferenceNamePlatform {
			if preference.Value == storemodels.PreferenceValuePlatformMM {
				usersWithMMPrimary = append(usersWithMMPrimary, preference.UserId)
			} else if preference.Value == storemodels.PreferenceValuePlatformMSTeams {
				usersWithTeamsPrimary = append(usersWithTeamsPrimary, preference.UserId)
			}
		}
	}

	return usersWithTeamsPrimary, usersWithMMPrimary
}
