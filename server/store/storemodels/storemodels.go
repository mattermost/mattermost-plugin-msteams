package storemodels

import (
	"errors"
	"time"
)

const (
	// The preference with this name stores the user's choice of primary platform.
	PreferenceNamePlatform         = "platform"
	PreferenceValuePlatformMM      = "mattermost"
	PreferenceValuePlatformMSTeams = "msteams"
)

type StatType string

const (
	StatsActiveUsersSending   StatType = "active_users_sending"
	StatsActiveUsersReceiving StatType = "active_users_receiving"
	StatsConnectedUsers       StatType = "connected_users"
	StatsSyntheticUsers       StatType = "synthetic_users"
	StatsLinkedChannels       StatType = "linked_channels"
	StatsPrimaryPlatform      StatType = "primary_platform"
)

type GetStatsOptions struct {
	RemoteID           string
	PreferenceCategory string
	ActiveUsersFrom    time.Time
	ActiveUsersTo      time.Time
	// What stats to retrieve. If empty, all stats are retrieved.
	Stats []StatType
}

func (o *GetStatsOptions) MustGetStat(s StatType) bool {
	if len(o.Stats) == 0 {
		return true
	}

	for _, v := range o.Stats {
		if v == s {
			return true
		}
	}
	return false
}

func (o *GetStatsOptions) IsValid() error {
	if o.RemoteID == "" && o.MustGetStat(StatsSyntheticUsers) {
		return errors.New("RemoteID is required")
	}

	if o.PreferenceCategory == "" && o.MustGetStat(StatsPrimaryPlatform) {
		return errors.New("PreferenceCategory is required")
	}

	if o.MustGetStat(StatsActiveUsersSending) || o.MustGetStat(StatsActiveUsersReceiving) || o.MustGetStat(StatsConnectedUsers) {
		if o.ActiveUsersFrom.IsZero() {
			return errors.New("ActiveUsersFrom is required")
		}

		if o.ActiveUsersTo.IsZero() {
			return errors.New("ActiveUsersTo is required")
		}

		if o.ActiveUsersFrom.After(o.ActiveUsersTo) {
			return errors.New("ActiveUsersFrom must be before ActiveUsersTo")
		}
	}

	return nil
}

type Stats struct {
	ActiveUsersSending   int64
	ActiveUsersReceiving int64
	ConnectedUsers       int64
	SyntheticUsers       int64
	LinkedChannels       int64
	MattermostPrimary    int64
	MSTeamsPrimary       int64
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
