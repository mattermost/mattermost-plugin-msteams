package main

import (
	"math"

	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) ChannelShouldSync(channelID string) (bool, error) {
	members, err := p.apiClient.Channel.ListMembers(channelID, 0, math.MaxInt32)
	if err != nil {
		return false, err
	}

	if len(members) == 1 {
		return true, nil
	}

	return p.MembersContainsRemote(members)
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
