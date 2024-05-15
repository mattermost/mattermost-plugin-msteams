package main

import (
	"math"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/pkg/errors"
)

func (p *Plugin) SelectiveSyncChannel(channelID, senderID string) (bool, error) {
	members, err := p.apiClient.Channel.ListMembers(channelID, 0, math.MaxInt32)
	if err != nil {
		return false, err
	}

	if len(members) == 1 {
		return true, nil
	}

	if p.GetSyncRemoteOnly() {
		if senderID != "" {
			return p.ChannelConnectedOrRemote(channelID, senderID)
		} else {
			return p.MembersContainsRemote(members)
		}

	} else {
		return p.ChatMembersSpanPlatforms(members)
	}
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

		mlog.Debug("ChatMembersSpanPlatforms user" + user.Username)
		if p.IsRemoteUser(user) {
			// Synthetic users are always remote.
			atLeastOneRemoteUser = true
		} else if p.getPrimaryPlatform(user.Id) == storemodels.PreferenceValuePlatformMSTeams {
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

func (p *Plugin) ChannelConnectedOrRemote(channelID, senderID string) (bool, error) {
	senderConnected, err := p.IsUserConnected(senderID)
	if err != nil {
		// ah.plugin.GetAPI().LogWarn("Unable to determine if sender is connected", "error", err.Error())
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
	mlog.Debug("MembersContainsRemote", mlog.Int("count", len(members)))
	for _, m := range members {
		user, err := p.apiClient.User.Get(m.UserId)
		if err != nil {
			return false, err
		}
		mlog.Debug("MembersContainsRemote - got user")

		if p.IsRemoteUser(user) {
			mlog.Debug("MembersContainsRemote true")
			return true, nil
		}
	}
	mlog.Debug("MembersContainsRemote false")
	return false, nil
}
