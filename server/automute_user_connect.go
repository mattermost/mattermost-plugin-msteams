package main

func (p *Plugin) updateAutomutingOnUserConnect(userID string) (bool, error) {
	if !p.isUsersPrimaryPlatformTeams(userID) {
		return false, nil
	}

	automuteEnabled, err := p.enableAutomute(userID)
	if err != nil {
		p.API.LogWarn(
			"Unable to enable automuting for user on connect",
			"user_id", userID,
			"error", err,
		)
	} else {
		var message string
		if automuteEnabled {
			message = "Enabled automuting for user on connect"
		} else {
			message = "Not enabling automute for user on connect"
		}

		p.API.LogInfo(
			message,
			"user_id", userID,
		)
	}

	return automuteEnabled, err
}

func (p *Plugin) updateAutomutingOnUserDisconnect(userID string) (bool, error) {
	automuteDisabled, err := p.disableAutomute(userID)
	if err != nil {
		p.API.LogWarn(
			"Unable to disable automuting for user on disconnect",
			"user_id", userID,
			"error", err,
		)

		return false, err
	}

	var message string
	if automuteDisabled {
		message = "Disabled automuting for user on disconnect"
	} else {
		message = "User disconnected without automuting enabled"
	}

	p.API.LogInfo(
		message,
		"user_id", userID,
	)

	return automuteDisabled, err
}
