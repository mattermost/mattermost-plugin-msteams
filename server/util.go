package main

import "github.com/mattermost/mattermost-server/v6/model"

func (p *Plugin) IsSystemAdmin(userID string) bool {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return false
	}

	return user.IsInRole(model.SystemAdminRoleId)
}

func (p *Plugin) IsChannelAdmin(userID, channelID string) bool {
	channelMember, err := p.API.GetChannelMember(channelID, userID)
	if err != nil {
		return false
	}

	return channelMember.SchemeAdmin
}
