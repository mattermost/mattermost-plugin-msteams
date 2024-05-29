package main

import (
	"math"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

func (p *Plugin) ChannelShouldSyncCreated(channelID, senderID string) (bool, error) {
	if senderID == "" {
		return false, errors.New("Invalid function call, requires RemoteOnly and a senderID")
	}

	if p.GetSyncRemoteOnly() {
		return p.ChannelConnectedOrRemote(channelID, senderID)
	}
	return p.ChannelShouldSync(channelID)
}

func (p *Plugin) ChannelShouldSync(channelID string) (bool, error) {
	members, err := p.apiClient.Channel.ListMembers(channelID, 0, math.MaxInt32)
	if err != nil {
		return false, err
	}

	if len(members) == 1 {
		return true, nil
	}

	if p.GetSyncRemoteOnly() {
		return p.MembersContainsRemote(members)
	}
	return p.ChatMembersSpanPlatforms(members)
}

// ChatMembersSpanPlatforms determines if the given channel members span both Mattermost and
// MS Teams. Chats between users on the same platform are skipped if selective sync is enabled.
func (p *Plugin) ChatMembersSpanPlatforms(members []*model.ChannelMember) (bool, error) {
	if len(members) == 1 {
		return false, errors.New("Invalid function call, requires multiple members")
	}
	atLeastOneLocalUser := false
	atLeastOneRemoteUser := false
	for _, m := range members {
		user, err := p.apiClient.User.Get(m.UserId)
		if err != nil {
			return false, err
		}

		if p.IsRemoteUser(user) {
			// Synthetic users are always remote.
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

func (p *Plugin) ChannelConnectedOrRemote(channelID, senderID string) (bool, error) {
	senderConnected, err := p.IsUserConnected(senderID)
	if err != nil {
		return false, err
	}
	members, err := p.apiClient.Channel.ListMembers(channelID, 0, math.MaxInt32)
	if err != nil {
		return false, err
	}
	if len(members) == 1 {
		return true, nil
	}

	if senderConnected {
		containsRemote, memberErr := p.MembersContainsRemote(members)
		if memberErr != nil {
			return false, memberErr
		}
		return containsRemote, nil
	}

	senderMember := &model.ChannelMember{
		UserId: senderID,
	}
	senderRemote, err := p.MembersContainsRemote([]*model.ChannelMember{senderMember})
	if err != nil {
		return false, err
	}
	if !senderRemote {
		return false, nil
	}
	for _, m := range members {
		isConnected, err := p.IsUserConnected(m.UserId)
		if err != nil {
			return false, err
		} else if isConnected {
			return true, nil
		}
	}
	return false, nil
}

// MembersContainsRemote determines if any of the given channel members are remote.
func (p *Plugin) MembersContainsRemote(members []*model.ChannelMember) (bool, error) {
	for _, m := range members {
		user, err := p.apiClient.User.Get(m.UserId)
		if err != nil {
			return false, err
		}

		if p.IsRemoteUser(user) {
			return true, nil
		}
	}
	return false, nil
}
