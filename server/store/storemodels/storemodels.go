package storemodels

import "time"

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
}

type ChatSubscription struct {
	SubscriptionID string
	UserID         string
	ExpiresOn      time.Time
	Secret         string
}

type ChannelSubscription struct {
	SubscriptionID string
	TeamID         string
	ChannelID      string
	ExpiresOn      time.Time
	Secret         string
}

type ConnectedUsers struct {
	MattermostUserID string
	TeamsUserID      string
}
