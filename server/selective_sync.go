package main

import (
	"math"

	"github.com/mattermost/mattermost/server/public/model"
)

// ChatMembersSpanPlatforms determines if the members of the given channel span both Mattermost and
// MS Teams. Chats between users on the same platform are skipped if selective sync is enabled.
func (p *Plugin) ChatSpansPlatforms(channelID string) (bool, *model.AppError) {
	members, appErr := p.API.GetChannelMembers(channelID, 0, math.MaxInt32)
	if appErr != nil {
		return false, appErr
	}

	return p.ChatMembersSpanPlatforms(members)
}

// ChatMembersSpanPlatforms determines if the given channel members span both Mattermost and
// MS Teams. Chats between users on the same platform are skipped if selective sync is enabled.
func (p *Plugin) ChatMembersSpanPlatforms(members model.ChannelMembers) (bool, *model.AppError) {
	atLeastOneLocalUser := false
	atLeastOneRemoteUser := false
	for _, m := range members {
		user, appErr := p.API.GetUser(m.UserId)
		if appErr != nil {
			return false, appErr
		}

		if p.IsRemoteUser(user) {
			// Synthetic users are always remote.
			atLeastOneRemoteUser = true
		} else if p.getAutomuteIsEnabledForUser(user.Id) {
			// Treat Teams primary users as remote
			atLeastOneRemoteUser = true
		} else {
			// Otherwise the user is considered local.
			atLeastOneLocalUser = true
		}

		if atLeastOneLocalUser && atLeastOneRemoteUser {
			return true, nil
		}
	}

	return false, nil
}
