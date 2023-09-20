package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

const (
	ErrorNoRowsInResult = "sql: no rows in result set"

	// Query params
	QueryParamPerPage = "per_page"
	QueryParamPage    = "page"

	// Pagination
	DefaultPage         = 0
	DefaultPerPageLimit = 10
	MaxPerPageLimit     = 100
)

func (p *Plugin) GetMSTeamsTeamAndChannelDetailsFromChannelLinks(channelLinks []*storemodels.ChannelLink, userID string, checkChannelPermissions bool) (map[string]string, map[string]string, map[string]string, bool) {
	msTeamsTeamIDsVsNames := make(map[string]string)
	msTeamsChannelIDsVsNames := make(map[string]string)
	msTeamsTeamIDsVsChannelsQuery := make(map[string]string)
	msTeamsChannelIDsVsType := make(map[string]string)

	for _, link := range channelLinks {
		if checkChannelPermissions && !p.API.HasPermissionToChannel(userID, link.MattermostChannelID, model.PermissionCreatePost) {
			p.API.LogInfo("User does not have the permissions for the requested channel", "UserID", userID, "ChannelID", link.MattermostChannelID)
			continue
		}

		msTeamsTeamIDsVsNames[link.MSTeamsTeamID] = ""
		msTeamsChannelIDsVsNames[link.MSTeamsChannelID] = ""
		msTeamsChannelIDsVsType[link.MSTeamsChannelID] = ""

		// Build the channels query for each team
		if msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeamID] == "" {
			msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeamID] = "id in ("
		} else if channelsQuery := msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeamID]; channelsQuery[len(channelsQuery)-1:] != "(" {
			msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeamID] += ","
		}

		msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeamID] += "'" + link.MSTeamsChannelID + "'"
	}

	errorsFound := false
	// Get MS Teams display names for each unique team ID and store it
	if p.GetMSTeamsTeamDetails(msTeamsTeamIDsVsNames) {
		errorsFound = true
	}

	// Get MS Teams channel details for all channels for each unique team
	if p.GetMSTeamsChannelDetailsForAllTeams(msTeamsTeamIDsVsChannelsQuery, msTeamsChannelIDsVsNames, msTeamsChannelIDsVsType) {
		errorsFound = true
	}

	return msTeamsTeamIDsVsNames, msTeamsChannelIDsVsNames, msTeamsChannelIDsVsType, errorsFound
}

func (p *Plugin) GetMSTeamsTeamDetails(msTeamsTeamIDsVsNames map[string]string) bool {
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

func (p *Plugin) GetMSTeamsChannelDetailsForAllTeams(msTeamsTeamIDsVsChannelsQuery, msTeamsChannelIDsVsNames, msTeamsChannelIDsVsType map[string]string) bool {
	errorsFound := false
	for teamID, channelsQuery := range msTeamsTeamIDsVsChannelsQuery {
		channels, err := p.msteamsAppClient.GetChannelsInTeam(teamID, channelsQuery+")")
		if err != nil {
			p.API.LogDebug("Unable to get the MS Teams channel information for the team", "TeamID", teamID, "Error", err.Error())
			errorsFound = true
			continue
		}

		for _, channel := range channels {
			msTeamsChannelIDsVsNames[channel.ID] = channel.DisplayName
			if channel.Type == models.PRIVATE_CHANNELMEMBERSHIPTYPE {
				msTeamsChannelIDsVsType[channel.ID] = string(model.ChannelTypePrivate)
			} else {
				msTeamsChannelIDsVsType[channel.ID] = string(model.ChannelTypeOpen)
			}
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

func (p *Plugin) GetMSTeamsTeamChannels(teamID, userID string) ([]msteams.Channel, int, error) {
	client, err := p.GetClientForUser(userID)
	if err != nil {
		p.API.LogError("Unable to get the client for user", "Error", err.Error())
		return nil, http.StatusUnauthorized, err
	}

	channels, err := client.ListChannels(teamID)
	if err != nil {
		p.API.LogError("Unable to get the channels for MS Teams team", "TeamID", teamID, "Error", err.Error())
		return nil, http.StatusInternalServerError, err
	}

	return channels, http.StatusOK, nil
}

func (p *Plugin) LinkChannels(userID, mattermostTeamID, mattermostChannelID, msTeamsTeamID, msTeamsChannelID string) (string, int) {
	channel, appErr := p.API.GetChannel(mattermostChannelID)
	if appErr != nil {
		p.API.LogError("Unable to get the current channel information.", "ChannelID", mattermostChannelID, "Error", appErr.Message)
		return "Unable to get the current channel information.", http.StatusInternalServerError
	}

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		p.API.LogError("Linking/unlinking a direct or group message is not allowed.", "ChannelType", channel.Type)
		return "Linking/unlinking a direct or group message is not allowed", http.StatusForbidden
	}

	if !p.API.HasPermissionToChannel(userID, mattermostChannelID, model.PermissionManageChannelRoles) {
		p.API.LogError("Unable to link the channel. You have to be a channel admin to link it.", "ChannelID", mattermostChannelID)
		return "Unable to link the channel. You have to be a channel admin to link it.", http.StatusForbidden
	}

	if !p.store.CheckEnabledTeamByTeamID(mattermostTeamID) {
		p.API.LogError("This team is not enabled for MS Teams sync.", "TeamID", mattermostTeamID)
		return "This team is not enabled for MS Teams sync.", http.StatusForbidden
	}

	link, err := p.store.GetLinkByChannelID(mattermostChannelID)
	if err != nil && err.Error() != ErrorNoRowsInResult {
		p.API.LogError("Error occurred while getting channel link with Mattermost channelID.", "ChannelID", mattermostChannelID, "Error", err.Error())
		return "Error occurred while getting channel link with Mattermost channelID.", http.StatusInternalServerError
	}
	if link != nil {
		return "A link for this channel already exists. Please unlink the channel before you link again with another channel.", http.StatusBadRequest
	}

	link, err = p.store.GetLinkByMSTeamsChannelID(msTeamsTeamID, msTeamsChannelID)
	if err != nil && err.Error() != ErrorNoRowsInResult {
		p.API.LogError("Error occurred while getting the channel link with MS Teams channel ID", "Error", err.Error())
		return "Error occurred while getting the channel link with MS Teams channel ID", http.StatusInternalServerError
	}
	if link != nil {
		return "A link for this channel already exists. Please unlink the channel before you link again with another channel.", http.StatusBadRequest
	}

	client, err := p.GetClientForUser(userID)
	if err != nil {
		p.API.LogError("Error occurred while getting client for the user.", "Error", err.Error())
		return "Unable to link the channel, looks like your account is not connected to MS Teams", http.StatusInternalServerError
	}

	if _, err = client.GetChannelInTeam(msTeamsTeamID, msTeamsChannelID); err != nil {
		p.API.LogError("Error occurred while getting channel in MS Teams team.", "Error", err.Error())
		return "MS Teams channel not found or you don't have the permissions to access it.", http.StatusInternalServerError
	}

	channelLink := storemodels.ChannelLink{
		MattermostTeamID:    channel.TeamId,
		MattermostChannelID: channel.Id,
		MSTeamsTeamID:       msTeamsTeamID,
		MSTeamsChannelID:    msTeamsChannelID,
		Creator:             userID,
	}
	if err = p.store.StoreChannelLink(&channelLink); err != nil {
		p.API.LogError("Error occurred while storing the channel link.", "Error", err.Error())
		return "Unable to create new link.", http.StatusInternalServerError
	}

	channelsSubscription, err := p.msteamsAppClient.SubscribeToChannel(channelLink.MSTeamsTeamID, channelLink.MSTeamsChannelID, p.GetURL()+"/", p.getConfiguration().WebhookSecret)
	if err != nil {
		p.API.LogError("Error occurred while subscribing to MS Teams channel", "TeamID", channelLink.MSTeamsTeamID, "ChannelID", channelLink.MSTeamsChannelID, "Error", err.Error())
		return "Unable to subscribe to the channel: " + err.Error(), http.StatusInternalServerError
	}

	if err = p.store.SaveChannelSubscription(storemodels.ChannelSubscription{
		SubscriptionID: channelsSubscription.ID,
		TeamID:         channelLink.MSTeamsTeamID,
		ChannelID:      channelLink.MSTeamsChannelID,
		ExpiresOn:      channelsSubscription.ExpiresOn,
		Secret:         p.getConfiguration().WebhookSecret,
	}); err != nil {
		p.API.LogError("Error occurred while saving the channel subscription.", "Error", err.Error())
		return "Unable to save the subscription in the monitoring system: " + err.Error(), http.StatusInternalServerError
	}

	return "", http.StatusOK
}

func (p *Plugin) UnlinkChannels(userID, mattermostChannelID string) (string, int) {
	channel, appErr := p.API.GetChannel(mattermostChannelID)
	if appErr != nil {
		p.API.LogError("Unable to get the current channel information.", "ChannelID", mattermostChannelID, "Error", appErr.Message)
		return "Unable to get the current channel information.", http.StatusInternalServerError
	}

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		p.API.LogError("Linking/unlinking a direct or group message is not allowed")
		return "Linking/unlinking a direct or group message is not allowed", http.StatusBadRequest
	}

	canLinkChannel := p.API.HasPermissionToChannel(userID, mattermostChannelID, model.PermissionManageChannelRoles)
	if !canLinkChannel {
		p.API.LogError("Unable to unlink the channel, you have to be a channel admin to unlink it.", "ChannelID", mattermostChannelID)
		return "Unable to unlink the channel, you have to be a channel admin to unlink it.", http.StatusForbidden
	}

	link, err := p.store.GetLinkByChannelID(channel.Id)
	if err != nil {
		p.API.LogError("This Mattermost channel is not linked to any MS Teams channel.", "ChannelID", channel.Id, "Error", err.Error())
		return "This Mattermost channel is not linked to any MS Teams channel.", http.StatusBadRequest
	}

	if err := p.store.DeleteLinkByChannelID(channel.Id); err != nil {
		p.API.LogError("Unable to delete link.", "Error", err.Error())
		return "Unable to delete link.", http.StatusInternalServerError
	}

	subscription, err := p.store.GetChannelSubscriptionByTeamsChannelID(link.MSTeamsChannelID)
	if err != nil {
		p.API.LogDebug("Unable to get the subscription by MS Teams channel ID", "error", err.Error())
		return "Unable to get the subscription by MS Teams channel ID", http.StatusInternalServerError
	}

	if err = p.store.DeleteSubscription(subscription.SubscriptionID); err != nil {
		p.API.LogDebug("Unable to delete the subscription from the DB", "subscriptionID", subscription.SubscriptionID, "error", err.Error())
		return "Unable to delete the subscription from the DB", http.StatusInternalServerError
	}

	if err = p.msteamsAppClient.DeleteSubscription(subscription.SubscriptionID); err != nil {
		p.API.LogDebug("Unable to delete the subscription on MS Teams", "subscriptionID", subscription.SubscriptionID, "error", err.Error())
		return "Unable to delete the subscription on MS Teams", http.StatusInternalServerError
	}

	return "", http.StatusOK
}

func (p *Plugin) GetOffsetAndLimitFromQueryParams(query url.Values) (offset, limit int) {
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
