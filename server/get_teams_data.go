package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"

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

func (p *Plugin) GetMSTeamsTeamAndChannelDetailsFromChannelLinks(channelLinks []*storemodels.ChannelLink, userID string, checkChannelPermissions bool) (msTeamsTeamIDsVsNames map[string]string, msTeamsChannelIDsVsNames map[string]string, errorsFound bool) {
	msTeamsTeamIDsVsChannelsQuery := make(map[string]string)
	msTeamsTeamIDsVsNames = make(map[string]string)
	msTeamsChannelIDsVsNames = make(map[string]string)

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
	errorsFound = p.GetMSTeamsTeamDetails(msTeamsTeamIDsVsNames)

	// Get MS Teams channel details for all channels for each unique team
	errorsFound = p.GetMSTeamsChannelDetailsForAllTeams(msTeamsTeamIDsVsChannelsQuery, msTeamsChannelIDsVsNames) || errorsFound

	return msTeamsTeamIDsVsNames, msTeamsChannelIDsVsNames, errorsFound
}

func (p *Plugin) GetMSTeamsTeamDetails(msTeamsTeamIDsVsNames map[string]string) (errorsFound bool) {
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
		return true
	}

	for _, msTeamsTeam := range msTeamsTeams {
		msTeamsTeamIDsVsNames[msTeamsTeam.ID] = msTeamsTeam.DisplayName
	}

	return false
}

func (p *Plugin) GetMSTeamsChannelDetailsForAllTeams(msTeamsTeamIDsVsChannelsQuery, msTeamsChannelIDsVsNames map[string]string) (errorsFound bool) {
	for teamID, channelsQuery := range msTeamsTeamIDsVsChannelsQuery {
		channels, err := p.msteamsAppClient.GetChannelsInTeam(teamID, channelsQuery+")")
		if err != nil {
			p.API.LogDebug("Unable to get the MS Teams channel information for the team", "TeamID", teamID, "Error", err.Error())
			errorsFound = true
			continue
		}

		for _, channel := range channels {
			msTeamsChannelIDsVsNames[channel.ID] = channel.DisplayName
		}
	}

	return errorsFound
}

func (p *Plugin) GetMSTeamsTeamList(userID string) ([]msteams.Team, int, error) {
	client, err := p.GetClientForUser(userID)
	if err != nil {
		p.API.LogError("Unable to get the client for user", "Error", err.Error())
		return nil, http.StatusUnauthorized, err
	}

	teams, err := client.ListTeams()
	if err != nil {
		p.API.LogError("Unable to get the MS Teams team list", "Error", err.Error())
		return nil, http.StatusInternalServerError, err
	}

	return teams, http.StatusOK, nil
}

func (p *Plugin) GetOffsetAndLimit(query url.Values) (offset, limit int) {
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
