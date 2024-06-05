package main

import (
	"math"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost/server/public/model"
)

// ChatShouldSync implements the check for selective sync to determine if we sync a direct message
// or group message. We avoid syncing messages between local users, otherwise we generate dual
// unreads and notifications. Thus, this method reduces to a check that there is at most one local
// user in the given channel, and at least one remote user.
//
// The return value of the second boolean parameter, containsRemoteUser, is only guaranteed to be
// set correctly if the value of the first boolean parameter, chatShouldSync, is true.
//
// TODO: This method does too much, but it's reflective of the underlying complexity of the
// business logic. Thankfully, it's well tested!
func (p *Plugin) ChatShouldSync(channel *model.Channel) (bool, bool, []*model.ChannelMember, string, error) {
	// Check for a DM or GM, and whether or not either has been disabled.
	if shouldSync, discardReason := p.ShouldSyncDMGMChannel(channel); !shouldSync {
		return false, false, nil, discardReason, nil
	}

	// We use the members to count the number of remote users, but also to return to the client
	// for subsequent use.
	members, err := p.apiClient.Channel.ListMembers(channel.Id, 0, math.MaxInt32)
	if err != nil {
		return false, false, nil, metrics.DiscardedReasonInternalError, err
	}

	numLocalUsers := 0
	numRemoteUsers := 0
	for _, m := range members {
		user, err := p.apiClient.User.Get(m.UserId)
		if err != nil {
			return false, false, nil, metrics.DiscardedReasonInternalError, err
		}

		if p.IsRemoteUser(user) {
			numRemoteUsers++
		} else {
			numLocalUsers++
		}
	}
	containsRemoteUser := numRemoteUsers > 0

	// If selective sync is disabled, there are no restrictions on syncing chats.
	if p.getConfiguration().SelectiveSync {
		// Only sync if there's at most one local user and at least one remote user.
		if numLocalUsers == 1 && containsRemoteUser {
			return true, containsRemoteUser, members, metrics.DiscardedReasonNone, nil
		}

		return false, containsRemoteUser, members, metrics.DiscardedReasonSelectiveSync, nil
	}

	return true, containsRemoteUser, members, metrics.DiscardedReasonNone, nil
}
