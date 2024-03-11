package main

import (
	"fmt"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
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
	now := time.Now()
	var pendingSince time.Time
	var lastSent time.Time
	user, _ := p.apiClient.User.Get(userID)

	userInWhitelist, err := p.store.IsUserPresentInWhitelist(user.Id)
	if err != nil {
		p.API.LogWarn("Error getting user in whitelist", "error", err.Error())
		return false, err
	}

	if userInWhitelist {
		// user already connected
		return false, nil
	}

	invitedUser, _ := p.store.GetInvitedUser(user.Id)

	if invitedUser != nil {
		pendingSince = invitedUser.InvitePendingSince
		lastSent = invitedUser.InviteLastSentAt
	} else {
		moreInvitesAllowed, err := p.moreInvitesAllowed(now)
		if err != nil {
			return false, errors.Wrapf(err, "Error checking if connection invite can be sent to user_id: %s", userID)
		}

		if !moreInvitesAllowed {
			// user not connected, but invite threshold already met
			return false, nil
		}
	}

	if !p.shouldSendInviteMessage(pendingSince, lastSent, now, user.GetTimezoneLocation()) {
		return false, nil
	}

	if err := p.SendInviteMessage(user.Id, pendingSince, now); err != nil {
		p.API.LogWarn("Error sending connection invite", "error", err.Error())
		return false, err
	}

	return true, nil
}

func (p *Plugin) SendInviteMessage(userID string, pendingSince time.Time, currentTime time.Time) error {
	invitedUser := &storemodels.InvitedUser{ID: userID, InvitePendingSince: pendingSince, InviteLastSentAt: currentTime}
	if invitedUser.InvitePendingSince.IsZero() {
		invitedUser.InvitePendingSince = currentTime
	}

	if err := p.store.StoreInvitedUser(invitedUser); err != nil {
		p.API.LogWarn("Error storing user in invite list", "error", err.Error())
		return err
	}

	connectURL := p.GetURL() + "/connect"

	return p.botSendDirectMessage(userID, fmt.Sprintf("You're invited to be an early adopter for the MS Teams connected experience. [Click here to connect your account](%s)", connectURL))
}

func (p *Plugin) shouldSendInviteMessage(
	pendingSince time.Time,
	lastSent time.Time,
	currentTime time.Time,
	timezone *time.Location,
) bool {
	firstSentTime := pendingSince.In(timezone)
	lastSentTime := lastSent.In(timezone)
	now := currentTime.In(timezone)

	currentYear, currentWeek := now.ISOWeek()
	currentlyWeekday := now.Weekday()
	lastSentYear, lastSentWeek := lastSentTime.ISOWeek()

	if currentlyWeekday == time.Saturday || currentlyWeekday == time.Sunday {
		// don't send on weekends
		return false
	}

	notSent := firstSentTime.IsZero()
	isFirstLoginOfTheWeek := currentYear != lastSentYear || currentWeek != lastSentWeek

	return notSent || isFirstLoginOfTheWeek
}

func (p *Plugin) moreInvitesAllowed(now time.Time) (bool, error) {
	nWhitelisted, err := p.store.GetSizeOfWhitelist()
	if err != nil {
		return false, errors.Wrapf(err, "Error in getting the size of whitelist")
	}
	nInvited, err := p.store.GetSizeOfInvitedUsers()
	if err != nil {
		return false, errors.Wrapf(err, "Error in getting the number of invited users")
	}

	unresponsiveCutoff := now.Add((-time.Duration(p.getConfiguration().ConnectedUsersInviteDaysUntilUnresponsive) * 24 * time.Hour))
	nUnresponsive, err := p.store.GetSizeOfUnresponsiveInvitedUsers(unresponsiveCutoff)
	if err != nil {
		return false, errors.Wrapf(err, "Error in getting the number of unresponsive invited users")
	}

	dailyThreshold := (p.getConfiguration().ConnectedUsersAllowed - nWhitelisted) / p.getConfiguration().ConnectedUsersInviteTimespanDays

	return (nInvited - nUnresponsive) < dailyThreshold, nil
}
