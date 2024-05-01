package main

import (
	"math"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// ChatShouldSync determines if the members of the given channel span both Mattermost and
// MS Teams. Chats between users on the same platform are skipped if selective sync is enabled.
// Chats with only a single member are self chats and always sync.
func (p *Plugin) ChatShouldSync(channelID string) (bool, *model.AppError) {
	members, appErr := p.API.GetChannelMembers(channelID, 0, math.MaxInt32)
	if appErr != nil {
		return false, appErr
	}
	if len(members) == 1 {
		return true, nil
	}

	return p.ChatMembersSpanPlatforms(members)
}

// ChatMembersSpanPlatforms determines if the given channel members span both Mattermost and
// MS Teams. Only Synthetic users are considered MS Teams users.
// Chats between users on the same platform are skipped if selective sync is enabled.
func (p *Plugin) ChatMembersSpanPlatforms(members model.ChannelMembers) (bool, *model.AppError) {
	if len(members) <= 1 {
		return false, &model.AppError{Message: "Invalid function call, requires multiple members"}
	}
	for _, m := range members {
		user, appErr := p.API.GetUser(m.UserId)
		if appErr != nil {
			mlog.Debug("got an error")
			return false, appErr
		}
		mlog.Debug("continue,", mlog.Bool("isnil", user == nil))

		if p.IsRemoteUser(user) {
			mlog.Debug("is remote")
			// Synthetic users are always remote.
			return true, nil
		}
	}

	return false, nil
}
