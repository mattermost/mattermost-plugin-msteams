package main

import (
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
	// TODO MM-56499
}
