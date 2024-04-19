package storemodels

import (
	"time"
)

const (
	// The preference with this name stores the user's choice of primary platform.
	PreferenceNamePlatform         = "platform"
	PreferenceValuePlatformMM      = "mattermost"
	PreferenceValuePlatformMSTeams = "msteams"
)

type Stats struct {
	ConnectedUsers    int64
	SyntheticUsers    int64
	LinkedChannels    int64
	MattermostPrimary int64
	MSTeamsPrimary    int64
}

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
