package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/model"
)

const (
	// Query params
	QueryParamPerPage = "per_page"
	QueryParamPage    = "page"

	// Pagination
	DefaultPage         = 0
	DefaultPerPageLimit = 10
	MaxPerPageLimit     = 100
)

func (p *Plugin) GetMSTeamsTeamAndChannelDetailsFromChannelLinks(channelLinks []*storemodels.ChannelLink, userID string, checkChannelPermissions bool) (map[string]string, map[string]string, error) {
	msTeamsTeamIDsVsNames := make(map[string]string)
	msTeamsChannelIDsVsNames := make(map[string]string)
	msTeamsTeamIDsVsChannelsQuery := make(map[string]string)

	for _, link := range channelLinks {
		if checkChannelPermissions && !p.API.HasPermissionToChannel(userID, link.MattermostChannelID, model.PermissionCreatePost) {
			p.API.LogInfo("User does not have the permissions for the requested channel", "UserID", userID, "ChannelID", link.MattermostChannelID)
			continue
		}

		msTeamsTeamIDsVsNames[link.MSTeamsTeamID] = ""
		msTeamsChannelIDsVsNames[link.MSTeamsChannelID] = ""

		// Build the channels query for each team
		if msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeamID] == "" {
			msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeamID] = "id in ("
		} else if channelsQuery := msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeamID]; channelsQuery[len(channelsQuery)-1:] != "(" {
			msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeamID] += ","
		}

		msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeamID] += "'" + link.MSTeamsChannelID + "'"
	}

	// Get MS Teams display names for each unique team ID and store it
	err := p.GetMSTeamsTeamDetails(msTeamsTeamIDsVsNames)

	// Get MS Teams channel details for all channels for each unique team
	err = p.GetMSTeamsChannelDetailsForAllTeams(msTeamsTeamIDsVsChannelsQuery, msTeamsChannelIDsVsNames)

	return msTeamsTeamIDsVsNames, msTeamsChannelIDsVsNames, err
}

func (p *Plugin) GetMSTeamsTeamDetails(msTeamsTeamIDsVsNames map[string]string) error {
	var msTeamsFilterQuery strings.Builder
	msTeamsFilterQuery.WriteString("id in (")
	for teamID := range msTeamsTeamIDsVsNames {
		msTeamsFilterQuery.WriteString("'" + teamID + "', ")
	}

	teamsQuery := msTeamsFilterQuery.String()
	teamsQuery = strings.TrimSuffix(teamsQuery, ", ") + ")"
	msTeamsTeams, err := p.msteamsAppClient.GetTeams(teamsQuery)
	if err != nil {
		p.API.LogDebug("Unable to get the MS Teams teams information", "Error", err.Error())
		return err
	}

	for _, msTeamsTeam := range msTeamsTeams {
		msTeamsTeamIDsVsNames[msTeamsTeam.ID] = msTeamsTeam.DisplayName
	}

	return nil
}

func (p *Plugin) GetMSTeamsChannelDetailsForAllTeams(msTeamsTeamIDsVsChannelsQuery, msTeamsChannelIDsVsNames map[string]string) error {
	for teamID, channelsQuery := range msTeamsTeamIDsVsChannelsQuery {
		channels, err := p.msteamsAppClient.GetChannelsInTeam(teamID, channelsQuery+")")
		if err != nil {
			p.API.LogDebug("Unable to get the MS Teams channel information for the team", "TeamID", teamID, "Error", err.Error())
			return err
		}

		for _, channel := range channels {
			msTeamsChannelIDsVsNames[channel.ID] = channel.DisplayName
		}
	}

	return nil
}

func (p *Plugin) GetOffsetAndLimitFromQueryParams(r *http.Request) (offset, limit int) {
	query := r.URL.Query()
	var page int
	if val, err := strconv.Atoi(query.Get(QueryParamPage)); err != nil || val < 0 {
		p.API.LogError("Invalid pagination query param", "Error", err.Error())
		page = DefaultPage
	} else {
		page = val
	}

	val, err := strconv.Atoi(query.Get(QueryParamPerPage))
	switch {
	case err != nil || val < 0:
		p.API.LogError("Invalid pagination query param", "Error", err.Error())
		limit = DefaultPerPageLimit
	case val > MaxPerPageLimit:
		p.API.LogInfo("Max per page limit reached")
		limit = MaxPerPageLimit
	default:
		limit = val
	}

	return page * limit, limit
}
