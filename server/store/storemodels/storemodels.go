package storemodels

import "time"

type ChannelLink struct {
	MattermostTeam    string
	MattermostChannel string
	MSTeamsTeam       string
	MSTeamsChannel    string
	Creator           string
}

type PostInfo struct {
	MattermostID        string
	MSTeamsID           string
	MSTeamsChannel      string
	MSTeamsLastUpdateAt time.Time
}
