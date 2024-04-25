package main

import (
	"fmt"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

const (
	NewConnectionsEnabled               = "enabled"
	NewConnectionsRolloutOpen           = "rolloutOpen"
	NewConnectionsRolloutOpenRestricted = "rolloutOpenRestricted"
)

func (p *Plugin) MaybeSendInviteMessage(userID string) (bool, error) {
	if p.getConfiguration().NewUserConnections == NewConnectionsEnabled {
		// new connections allowed, but invites disabled
		return false, nil
	}

	user, err := p.apiClient.User.Get(userID)
	if err != nil {
		return false, errors.Wrapf(err, "error getting user")
	}

	if p.getConfiguration().NewUserConnections == NewConnectionsRolloutOpenRestricted {
		// new connections allowed, but invites restricted to whitelist
		isWhitelisted, whitelistErr := p.store.IsUserWhitelisted(userID)
		if whitelistErr != nil {
			return false, errors.Wrapf(whitelistErr, "error getting user in whitelist")
		}

		if !isWhitelisted {
			return false, nil
		}
	}

	p.connectClusterMutex.Lock()
	defer p.connectClusterMutex.Unlock()

	hasConnected, err := p.store.UserHasConnected(user.Id)
	if err != nil {
		return false, errors.Wrapf(err, "error checking user connected status")
	}

	if hasConnected {
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

	channel, err := p.apiClient.Channel.GetDirect(user.Id, p.userID)
	if err != nil {
		return errors.Wrapf(err, "failed to get bot DM channel with user_id %s", user.Id)
	}

	message := fmt.Sprintf("@%s, you’re invited to use the Microsoft Teams connected experience for Mattermost. ", user.Username)
	invitePost := &model.Post{
		Message:   message,
		UserId:    p.userID,
		ChannelId: channel.Id,
	}
	if err := p.apiClient.Post.CreatePost(invitePost); err != nil {
		return errors.Wrapf(err, "error sending bot message")
	}

	connectURL := fmt.Sprintf(p.GetURL()+"/connect?post_id=%s&channel_id=%s", invitePost.Id, channel.Id)
	invitePost.Message = fmt.Sprintf("%s [Click here to activate the integration in a minute or less](%s). For best results, follow the prompts to pick your primary app and then disable notifications for the other app.", invitePost.Message, connectURL)
	if err := p.apiClient.Post.UpdatePost(invitePost); err != nil {
		return errors.Wrapf(err, "error sending bot message")
	}

	if err := p.store.StoreInvitedUser(invitedUser); err != nil {
		return errors.Wrapf(err, "error storing user in invite list")
	}

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
	nConnected, err := p.store.GetHasConnectedCount()
	if err != nil {
		return false, 0, errors.Wrapf(err, "error in getting has-connected count")
	}
	nInvited, err := p.store.GetInvitedCount()
	if err != nil {
		return false, 0, errors.Wrapf(err, "error in getting invited count")
	}

	if (nConnected + nInvited) >= p.getConfiguration().ConnectedUsersAllowed {
		// only invite up to max connected
		return false, 0, nil
	}

	return nInvited < p.getConfiguration().ConnectedUsersInvitePoolSize, nConnected, nil
}

func (p *Plugin) UserHasRightToConnect(mmUserID string) (bool, error) {
	hasConnected, err := p.store.UserHasConnected(mmUserID)
	if err != nil {
		return false, errors.Wrapf(err, "error in checking if user has connected or not")
	}

	if hasConnected {
		return true, nil
	}

	invitedUser, err := p.store.GetInvitedUser(mmUserID)
	if err != nil {
		return false, errors.Wrapf(err, "error in getting user invite")
	}

	if invitedUser != nil {
		return true, nil
	}

	return false, nil
}

func (p *Plugin) UserCanOpenlyConnect(mmUserID string) (bool, error) {
	numHasConnected, err := p.store.GetHasConnectedCount()
	if err != nil {
		return false, errors.Wrapf(err, "error in getting has connected count")
	}

	numInvited, err := p.store.GetInvitedCount()
	if err != nil {
		return false, errors.Wrapf(err, "error in getting invited count")
	}

	if (numHasConnected + numInvited) >= p.getConfiguration().ConnectedUsersAllowed {
		return false, nil
	}

	return true, nil
}
