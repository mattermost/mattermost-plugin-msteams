package main

import "github.com/mattermost/mattermost-server/v6/model"

func (p *Plugin) IsSystemAdmin(userID string) (bool, error) {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return false, appErr
	}

	return user.IsInRole(model.SystemAdminRoleId), nil
}

func (p *Plugin) IsChannelAdmin(userID, channelID string) (bool, error) {
	channelMember, err := p.API.GetChannelMember(channelID, userID)
	if err != nil {
		return false, err
	}

	return channelMember.SchemeAdmin, nil
}
