package storemodels

import "time"

type ChannelLink struct {
	MattermostTeamID      string `json:"mattermostTeamID,omitempty"`
	MattermostTeamName    string `json:"mattermostTeamName,omitempty"`
	MattermostChannelID   string `json:"mattermostChannelID,omitempty"`
	MattermostChannelName string `json:"mattermostChannelName,omitempty"`
	MSTeamsTeamID         string `json:"msTeamsTeamID,omitempty"`
	MSTeamsChannelID      string `json:"msTeamsChannelID,omitempty"`
	MSTeamsTeamName       string `json:"msTeamsTeamName,omitempty"`
	MSTeamsChannelName    string `json:"msTeamsChannelName,omitempty"`
	Creator               string `json:"creator,omitempty"`
}

type PostInfo struct {
	MattermostID        string
	MSTeamsID           string
	MSTeamsChannelID    string
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
