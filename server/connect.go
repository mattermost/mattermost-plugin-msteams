package main

import (
	"fmt"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

func (p *Plugin) MaybeSendInviteMessage(userID string) (bool, error) {
	if p.getConfiguration().ConnectedUsersInvitePoolSize == 0 {
		// connection invites disabled
		return false, nil
	}

	user, err := p.apiClient.User.Get(userID)
	if err != nil {
		return false, errors.Wrapf(err, "error getting user")
	}

	p.whitelistClusterMutex.Lock()
	defer p.whitelistClusterMutex.Unlock()

	userInWhitelist, err := p.store.IsUserPresentInWhitelist(user.Id)
	if err != nil {
		return false, errors.Wrapf(err, "error getting user in whitelist")
	}

	if userInWhitelist {
		// user already connected
		return false, nil
	}

	invitedUser, err := p.store.GetInvitedUser(user.Id)
	if err != nil {
		return false, errors.Wrapf(err, "error getting user invite")
	}

	var nWhitelisted int
	var pendingSince time.Time
	now := time.Now()

	if invitedUser != nil {
		pendingSince = invitedUser.InvitePendingSince
	} else {
		moreInvitesAllowed, n, err := p.moreInvitesAllowed()
		if err != nil {
			return false, errors.Wrapf(err, "error checking invite pool size")
		}

		if !moreInvitesAllowed {
			// user not connected, but invite threshold is presently met
			return false, nil
		}

		nWhitelisted = n
	}

	if !p.shouldSendInviteMessage(pendingSince, now, user.GetTimezoneLocation()) {
		return false, nil
	}

	if err := p.SendInviteMessage(user, pendingSince, now, nWhitelisted); err != nil {
		return false, errors.Wrapf(err, "error sending invite")
	}

	return true, nil
}

func (p *Plugin) SendInviteMessage(user *model.User, pendingSince time.Time, currentTime time.Time, nWhitelisted int) error {
	invitedUser := &storemodels.InvitedUser{ID: user.Id, InvitePendingSince: pendingSince, InviteLastSentAt: currentTime}
	if invitedUser.InvitePendingSince.IsZero() {
		invitedUser.InvitePendingSince = currentTime
	}

	if err := p.store.StoreInvitedUser(invitedUser); err != nil {
		return errors.Wrapf(err, "error storing user in invite list")
	}

	channel, err := p.apiClient.Channel.GetDirect(user.Id, p.userID)
	if err != nil {
		return errors.Wrapf(err, "failed to get bot DM channel with user_id %s", user.Id)
	}
	message := fmt.Sprintf("@%s, you're invited to use the MS Teams connected experience. ", user.Username)
	p.SendConnectMessage(channel.Id, user.Id, message)

	return nil
}

func (p *Plugin) shouldSendInviteMessage(
	pendingSince time.Time,
	currentTime time.Time,
	timezone *time.Location,
) bool {
	now := currentTime.In(timezone)

	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		// don't send on weekends
		return false
	}

	if !pendingSince.IsZero() {
		// only send once
		return false
	}

	return true
}

func (p *Plugin) moreInvitesAllowed() (bool, int, error) {
	nWhitelisted, err := p.store.GetSizeOfWhitelist()
	if err != nil {
		return false, 0, errors.Wrapf(err, "error in getting the size of whitelist")
	}
	nInvited, err := p.store.GetSizeOfInvitedUsers()
	if err != nil {
		return false, 0, errors.Wrapf(err, "error in getting the number of invited users")
	}

	if (nWhitelisted + nInvited) >= p.getConfiguration().ConnectedUsersAllowed {
		// only invite up to max connected
		return false, 0, nil
	}

	return nInvited < p.getConfiguration().ConnectedUsersInvitePoolSize, nWhitelisted, nil
}
