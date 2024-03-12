package main

import (
	"fmt"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

const (
	earlyAdopterThreshold = 1000
)

func (p *Plugin) botSendDirectMessage(userID, message string) error {
	channel, err := p.apiClient.Channel.GetDirect(userID, p.userID)
	if err != nil {
		return errors.Wrapf(err, "failed to get bot DM channel with user_id %s", userID)
	}

	return p.apiClient.Post.CreatePost(&model.Post{
		Message:   message,
		UserId:    p.userID,
		ChannelId: channel.Id,
	})
}

func (p *Plugin) MaybeSendInviteMessage(userID string) (bool, error) {
	if p.getConfiguration().ConnectedUsersInvitePoolSize == 0 {
		// connection invites disabled
		return false, nil
	}
	var nWhitelisted int
	var pendingSince time.Time
	now := time.Now()

	user, err := p.apiClient.User.Get(userID)
	if err != nil {
		p.API.LogWarn("Error getting user", "user_id", userID, "error", err.Error())
		return false, err
	}

	userInWhitelist, err := p.store.IsUserPresentInWhitelist(user.Id)
	if err != nil {
		p.API.LogWarn("Error getting user in whitelist", "user_id", userID, "error", err.Error())
		return false, err
	}

	if userInWhitelist {
		// user already connected
		return false, nil
	}

	invitedUser, err := p.store.GetInvitedUser(user.Id)
	if err != nil {
		p.API.LogWarn("Error getting user invite", "user_id", userID, "error", err.Error())
		return false, err
	}

	if invitedUser != nil {
		pendingSince = invitedUser.InvitePendingSince
	} else {
		moreInvitesAllowed, n, err := p.moreInvitesAllowed()
		if err != nil {
			p.API.LogWarn("Error checking invite pool size", "error", err.Error())
			return false, errors.Wrapf(err, "Error checking invite pool size")
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
		p.API.LogWarn("Error sending connection invite", "error", err.Error())
		return false, err
	}

	return true, nil
}

func (p *Plugin) SendInviteMessage(user *model.User, pendingSince time.Time, currentTime time.Time, nWhitelisted int) error {
	invitedUser := &storemodels.InvitedUser{ID: user.Id, InvitePendingSince: pendingSince, InviteLastSentAt: currentTime}
	if invitedUser.InvitePendingSince.IsZero() {
		invitedUser.InvitePendingSince = currentTime
	}

	if err := p.store.StoreInvitedUser(invitedUser); err != nil {
		p.API.LogWarn("Error storing user in invite list", "error", err.Error())
		return err
	}

	connectURL := p.GetURL() + "/connect"

	var message string
	if nWhitelisted < earlyAdopterThreshold {
		message = fmt.Sprintf("@%s, you're invited to be an early adopter for the MS Teams connected experience. [Click here to connect your account](%s).", user.Username, connectURL)
	} else {
		message = fmt.Sprintf("@%s, you're invited to use the MS Teams connected experience. [Click here to connect your account](%s).", user.Username, connectURL)
	}
	return p.botSendDirectMessage(user.Id, message)
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
		return false, 0, errors.Wrapf(err, "Error in getting the size of whitelist")
	}
	nInvited, err := p.store.GetSizeOfInvitedUsers()
	if err != nil {
		return false, 0, errors.Wrapf(err, "Error in getting the number of invited users")
	}

	if (nWhitelisted + nInvited) >= p.getConfiguration().ConnectedUsersAllowed {
		// only invite up to max connected
		return false, 0, nil
	}

	return nInvited < p.getConfiguration().ConnectedUsersInvitePoolSize, nWhitelisted, nil
}
