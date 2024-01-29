package main

func (p *Plugin) updateAutomutingOnUserConnect(userID string) (bool, error) {
	if !p.isUsersPrimaryPlatformTeams(userID) {
		return false, nil
	}

	automuteEnabled, err := p.enableAutomute(userID)
	if err != nil {
		p.API.LogError(
			"Unable to enable automuting for user on connect",
			"UserID", userID,
			"Error", err,
		)
	} else {
		var message string
		if automuteEnabled {
			message = "Enabled automuting for user on connect"
		} else {
			message = "Not enabling automute for user on connect"
		}

		p.API.LogDebug(
			message,
			"UserID", userID,
		)
	}

	return automuteEnabled, err
}

func (p *Plugin) updateAutomutingOnUserDisconnect(userID string) (bool, error) {
	automuteDisabled, err := p.disableAutomute(userID)
	if err != nil {
		p.API.LogError(
			"Unable to disable automuting for user on disconnect",
			"UserID", userID,
			"Error", err,
		)
	} else {
		var message string
		if automuteDisabled {
			message = "Disabled automuting for user on disconnect"
		} else {
			message = "User disconnected without automuting enabled"
		}

		p.API.LogDebug(
			message,
			"UserID", userID,
		)
	}

	return automuteDisabled, err
}
