package main

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/pkg/errors"
)

func (p *Plugin) UserHasJoinedChannel(c *plugin.Context, channelMember *model.ChannelMember, actor *model.User) {
	_, _ = p.updateAutomutingOnUserJoinedChannel(c, channelMember.UserId, channelMember.ChannelId)
}

func (p *Plugin) updateAutomutingOnUserJoinedChannel(_ *plugin.Context, userID string, channelID string) (bool, error) {
	if automuteEnabled := p.getAutomuteIsEnabledForUser(userID); !automuteEnabled {
		return false, nil
	}

	if canAutomute, err := p.canAutomuteChannelID(channelID); err != nil {
		p.API.LogError(
			"Unable to check if channel is linked to update automuting when a user has joined the channel",
			"user_id", userID,
			"channel_id", channelID,
			"error", err.Error(),
		)
		return false, errors.Wrap(err, "Unable to update automuting when a user has joined a channel")
	} else if !canAutomute {
		// Only automute channels that are linked
		return false, nil
	}

	err := p.setChannelMembersAutomuted([]*model.ChannelMemberIdentifier{{UserId: userID, ChannelId: channelID}}, true)
	return err == nil, err
}

func (p *Plugin) updateAutomutingOnChannelCreated(channel *model.Channel) error {
	if !channel.IsGroupOrDirect() {
		// Assume that newly created channels can never be linked by the time this is called
		return nil
	}

	return p.updateAutomutingForChannelMembers(channel.Id, true)
}

func (p *Plugin) updateAutomutingOnChannelLinked(channelID string) error {
	// This simply mutes the channel for all users with automuting enabled, regardless of their settings before. It
	// doesn't pay attention to if the user manually muted the channel beforehand.
	return p.updateAutomutingForChannelMembers(channelID, true)
}

func (p *Plugin) updateAutomutingOnChannelUnlinked(channelID string) error {
	// This simply unmutes the channel for all users with automuting enabled, regardless of their settings before. It
	// doesn't pay attention to if the user manually muted the channel beforehand to keep it muted.
	return p.updateAutomutingForChannelMembers(channelID, false)
}

func (p *Plugin) updateAutomutingForChannelMembers(channelID string, enableAutomute bool) error {
	var membersToUpdate []*model.ChannelMemberIdentifier

	page := 0
	perPage := 200
	for {
		members, appErr := p.API.GetChannelMembers(channelID, page, perPage)
		if appErr != nil {
			return errors.Wrap(appErr, fmt.Sprintf("Unable to get all members of channel %s to update automuting", channelID))
		}

		for _, member := range members {
			if p.getAutomuteIsEnabledForUser(member.UserId) {
				membersToUpdate = append(membersToUpdate, &model.ChannelMemberIdentifier{ChannelId: channelID, UserId: member.UserId})
			}
		}

		if len(members) < perPage {
			break
		}

		page += 1
	}

	if len(membersToUpdate) > 0 {
		if err := p.setChannelMembersAutomuted(membersToUpdate, enableAutomute); err != nil {
			return err
		}
	}

	return nil
}
