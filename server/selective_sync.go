package main

import (
	"math"

	"github.com/mattermost/mattermost/server/public/model"
)

// ChatShouldSync determines if the members of the given channel span both Mattermost and
// MS Teams. Chats between users on the same platform are skipped if selective sync is enabled.
// Chats with only a single member are self chats and always sync.
func (p *Plugin) ChatShouldSync(channelID string) (bool, error) {
	members, err := p.apiClient.Channel.ListMembers(channelID, 0, math.MaxInt32)
	if err != nil {
		return false, err
	}
	if len(members) == 1 {
		return true, nil
	}

	return p.ChatMembersSpanPlatforms(members)
}

// ChatMembersSpanPlatforms determines if the given channel members span both Mattermost and
// MS Teams. Only Synthetic users are considered MS Teams users.// Chats between users on the same platform are skipped if selective sync is enabled.
func (p *Plugin) ChatMembersSpanPlatforms(members []*model.ChannelMember) (bool, error) {
	if len(members) <= 1 {
		return false, &model.AppError{Message: "Invalid function call, requires multiple members"}
	}
	for _, m := range members {
		user, err := p.apiClient.User.Get(m.UserId)
		if err != nil {
			return false, err
		}

		if p.IsRemoteUser(user) {
			// Synthetic users are always remote.
			return true, nil
		}
	}

	return false, nil
}
