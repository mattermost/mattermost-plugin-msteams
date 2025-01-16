// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storemodels

import (
	"time"
)

const (
	// Notification preferences
	PreferenceNameNotification     = "notifications"
	PreferenceValueNotificationOn  = "on"
	PreferenceValueNotificationOff = "off"
)

type ChannelLink struct {
	MattermostTeamID      string
	MattermostTeamName    string
	MattermostChannelID   string
	MattermostChannelName string
	MSTeamsTeam           string
	MSTeamsChannel        string
	Creator               string
}

type PostInfo struct {
	MattermostID        string
	MSTeamsID           string
	MSTeamsChannel      string
	MSTeamsLastUpdateAt time.Time
}

type GlobalSubscription struct {
	SubscriptionID string
	Type           string
	ExpiresOn      time.Time
	Secret         string
	Certificate    string
}

type ChatSubscription struct {
	SubscriptionID string
	UserID         string
	ExpiresOn      time.Time
	Secret         string
	Certificate    string
}

type ChannelSubscription struct {
	SubscriptionID string
	TeamID         string
	ChannelID      string
	ExpiresOn      time.Time
	Secret         string
	Certificate    string
}

type ConnectedUser struct {
	MattermostUserID string
	TeamsUserID      string
	FirstName        string
	LastName         string
	Email            string
}

type UserConnectStatus struct {
	ID               string
	Connected        bool
	LastConnectAt    time.Time
	LastDisconnectAt time.Time
}

type InvitedUser struct {
	ID                 string
	InvitePendingSince time.Time
	InviteLastSentAt   time.Time
}

func MilliToMicroSeconds(milli int64) int64 {
	return milli * 1000
}
