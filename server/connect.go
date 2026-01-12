// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"time"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
)

func (p *Plugin) MaybeSendInviteMessage(userID string, currentTime time.Time) (bool, error) {
	if p.getConfiguration().ConnectedUsersMaxPendingInvites == 0 {
		return false, nil
	}

	if userID == p.botUserID {
		return false, errors.New("cannot invite plugin bot")
	}

	user, err := p.apiClient.User.Get(userID)
	if err != nil {
		return false, errors.Wrapf(err, "error getting user")
	}

	if user.IsBot {
		return false, errors.Wrapf(err, "bot accounts cannot be invited")
	}

	if user.IsGuest() {
		return false, errors.Wrapf(err, "guest accounts cannot be invited")
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

	var pendingSince time.Time

	if invitedUser != nil {
		pendingSince = invitedUser.InvitePendingSince
	} else {
		canInvite, err := p.canInviteUser(user.Id)
		if err != nil {
			return false, errors.Wrapf(err, "error checking if can invite")
		}

		if !canInvite {
			return false, nil
		}
	}

	if !p.shouldSendInviteMessage(pendingSince, currentTime, user.GetTimezoneLocation()) {
		return false, nil
	}

	if err := p.SendInviteMessage(user); err != nil {
		return false, errors.Wrapf(err, "error sending invite")
	}

	invitedUser = &storemodels.InvitedUser{
		ID:                 user.Id,
		InvitePendingSince: pendingSince,
		InviteLastSentAt:   currentTime,
	}
	if invitedUser.InvitePendingSince.IsZero() {
		invitedUser.InvitePendingSince = currentTime
	}
	if err := p.store.StoreInvitedUser(invitedUser); err != nil {
		return false, errors.Wrapf(err, "error storing user in invite list")
	}

	p.apiClient.Log.Info("Recorded user invite", "user_id", invitedUser.ID, "pending_since", invitedUser.InvitePendingSince, "last_sent_at", invitedUser.InviteLastSentAt)

	return true, nil
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

func (p *Plugin) canInviteUser(userID string) (bool, error) {
	if p.getConfiguration().ConnectedUsersRestricted {
		isWhitelisted, err := p.store.IsUserWhitelisted(userID)
		if err != nil {
			return false, errors.Wrapf(err, "error in checking if user is whitelisted")
		}

		if !isWhitelisted {
			// only whitelisted users can connect in restricted mode
			return false, nil
		}
	}

	nConnected, err := p.store.GetHasConnectedCount()
	if err != nil {
		return false, errors.Wrapf(err, "error in getting has-connected count")
	}
	nInvited, err := p.store.GetInvitedCount()
	if err != nil {
		return false, errors.Wrapf(err, "error in getting invited count")
	}

	if (nConnected + nInvited) >= p.getConfiguration().ConnectedUsersAllowed {
		// only invite up to max connected
		return false, nil
	}

	return nInvited < p.getConfiguration().ConnectedUsersMaxPendingInvites, nil
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

func (p *Plugin) UserCanOpenlyConnect(mmUserID string) (bool, int, error) {
	nConnected, err := p.store.GetHasConnectedCount()
	if err != nil {
		return false, 0, errors.Wrapf(err, "error in getting has connected count")
	}

	nInvited, err := p.store.GetInvitedCount()
	if err != nil {
		return false, 0, errors.Wrapf(err, "error in getting invited count")
	}

	nAvailable := p.getConfiguration().ConnectedUsersAllowed - nConnected - nInvited

	if p.getConfiguration().ConnectedUsersRestricted {
		isWhitelisted, err := p.store.IsUserWhitelisted(mmUserID)
		if err != nil {
			return false, 0, errors.Wrapf(err, "error in checking if user is whitelisted")
		}

		if !isWhitelisted {
			// only whitelisted users can connect in restricted mode
			return false, nAvailable, nil
		}
	}

	return nAvailable > 0, nAvailable, nil
}
