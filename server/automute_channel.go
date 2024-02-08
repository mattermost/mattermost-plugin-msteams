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

func (p *Plugin) updateAutomutingOnUserJoinedChannel(c *plugin.Context, userID string, channelID string) (bool, error) {
	if automuteEnabled := p.getAutomuteIsEnabledForUser(userID); !automuteEnabled {
		return false, nil
	}

	if canAutomute, err := p.isChannelLinked(channelID); err != nil {
		p.API.LogError(
			"Unable to check if channel is linked to update automuting when a user has joined the channel",
			"UserID", userID,
			"ChannelID", channelID,
			"Error", err.Error(),
		)
		return false, errors.Wrap(err, "Unable to update automuting when a user has joined a channel")
	} else if !canAutomute {
		// Only automute channels that are linked
		return false, nil
	}

	err := p.setChannelMembersAutomuted([]*model.ChannelMemberIdentifier{{UserId: userID, ChannelId: channelID}}, true)
	return err == nil, err
}

func (p *Plugin) ChannelHasBeenCreated(c *plugin.Context, channel *model.Channel) {
	_ = p.updateAutomutingOnChannelCreated(channel)
}

func (p *Plugin) updateAutomutingOnChannelCreated(channel *model.Channel) error {
	if !channel.IsGroupOrDirect() {
		// Assume that newly created channels can never be linked by the time this is called
		return nil
	}

	var membersToMute []*model.ChannelMemberIdentifier

	// Only get a single page of channel members since DMs/GMs can never have that many members
	members, appErr := p.API.GetChannelMembers(channel.Id, 0, 200)
	if appErr != nil {
		return errors.Wrap(appErr, fmt.Sprintf("Unable to get members of channel %s to automute them", channel.Id))
	}

	for _, member := range members {
		if p.getAutomuteIsEnabledForUser(member.UserId) {
			membersToMute = append(membersToMute, &model.ChannelMemberIdentifier{ChannelId: channel.Id, UserId: member.UserId})
		}
	}

	if len(membersToMute) > 0 {
		if err := p.setChannelMembersAutomuted(membersToMute, true); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Unable to mute members for newly created channel %s", channel.Id))
		}
	}

	return nil
}
