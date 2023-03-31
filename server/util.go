package main

import "github.com/mattermost/mattermost-server/v6/model"

func (p *Plugin) IsSystemAdmin(userID string) (bool, error) {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		p.API.LogError("Unable to get the user", "UserID", userID, "Error", appErr.Error())
		return false, appErr
	}

	return user.IsInRole(model.SystemAdminRoleId), nil
}

func (p *Plugin) IsChannelAdmin(userID, channelID string) (bool, error) {
	channelMember, err := p.API.GetChannelMember(channelID, userID)
	if err != nil {
		p.API.LogError("Unable to get the channel member", "UserID", userID, "ChannelID", channelID, "Error", err.Error())
		return false, err
	}

	return channelMember.SchemeAdmin, nil
}
