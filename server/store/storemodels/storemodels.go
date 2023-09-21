package storemodels

import (
	"errors"
	"time"
)

type Stats struct {
	ConnectedUsers int64
	SyntheticUsers int64
	LinkedChannels int64
}

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
	MattermostChannelType string `json:"mattermostChannelType,omitempty"`
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

type ConnectedUser struct {
	MattermostUserID string
	TeamsUserID      string
	FirstName        string
	LastName         string
	Email            string
}

func IsChannelLinkPayloadValid(body *ChannelLink) error {
	if body == nil {
		return errors.New("invalid body")
	}
	if body.MattermostTeamID == "" {
		return errors.New("mattermost team ID is required")
	}
	if body.MattermostChannelID == "" {
		return errors.New("mattermost channel ID is required")
	}
	if body.MSTeamsTeamID == "" {
		return errors.New("ms teams team ID is required")
	}
	if body.MSTeamsChannelID == "" {
		return errors.New("ms teams channel ID is required")
	}
	return nil
}
