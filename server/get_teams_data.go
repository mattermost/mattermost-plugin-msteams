package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/model"
)

const (
	ErrorNoRowsInResult = "sql: no rows in result set"
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
		p.API.LogDebug("Unable to get the MS Teams teams details", "Error", err.Error())
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
			p.API.LogDebug("Unable to get the MS Teams channel details for the team", "TeamID", teamID, "Error", err.Error())
			errorsFound = true
			continue
		}

		for _, channel := range channels {
			msTeamsChannelIDsVsNames[channel.ID] = channel.DisplayName
		}
	}

	return errorsFound
}

func (p *Plugin) GetMSTeamsTeamList(client msteams.Client) ([]*clientmodels.Team, int, error) {
	teams, err := client.ListTeams()
	if err != nil {
		p.API.LogError("Unable to get the MS Teams teams", "Error", err.Error())
		return nil, http.StatusInternalServerError, err
	}

	return teams, http.StatusOK, nil
}

func (p *Plugin) GetMSTeamsTeamChannels(teamID string, client msteams.Client) ([]*clientmodels.Channel, int, error) {
	channels, err := client.ListChannels(teamID)
	if err != nil {
		p.API.LogError("Unable to get the channels for MS Teams team", "TeamID", teamID, "Error", err.Error())
		return nil, http.StatusInternalServerError, err
	}

	return channels, http.StatusOK, nil
}

func (p *Plugin) LinkChannels(userID, mattermostTeamID, mattermostChannelID, msTeamsTeamID, msTeamsChannelID string, client msteams.Client) (responseMsg string, statusCode int) {
	channel, appErr := p.API.GetChannel(mattermostChannelID)
	if appErr != nil {
		p.API.LogError("Unable to get the current channel details.", "ChannelID", mattermostChannelID, "Error", appErr.Message)
		return "Unable to get the current channel details.", http.StatusInternalServerError
	}

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		p.API.LogError("Linking/unlinking a direct or group message is not allowed.", "ChannelType", channel.Type)
		return "Linking/unlinking a direct or group message is not allowed", http.StatusBadRequest
	}

	if !p.API.HasPermissionToChannel(userID, mattermostChannelID, model.PermissionManageChannelRoles) {
		p.API.LogError("Unable to link the channel. You have to be a channel admin to link it.", "ChannelID", mattermostChannelID)
		return "Unable to link the channel. You have to be a channel admin to link it.", http.StatusForbidden
	}

	if !p.store.CheckEnabledTeamByTeamID(mattermostTeamID) {
		p.API.LogError("This team is not enabled for MS Teams sync.", "TeamID", mattermostTeamID)
		return "This team is not enabled for MS Teams sync.", http.StatusBadRequest
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
		return "The Teams channel that you're trying to link is already linked to another Mattermost channel. Please unlink that channel and try again.", http.StatusBadRequest
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

	channelsSubscription, err := p.msteamsAppClient.SubscribeToChannel(channelLink.MSTeamsTeamID, channelLink.MSTeamsChannelID, p.GetURL()+"/", p.getConfiguration().WebhookSecret)
	if err != nil {
		p.API.LogError("Error occurred while subscribing to MS Teams channel", "TeamID", channelLink.MSTeamsTeamID, "ChannelID", channelLink.MSTeamsChannelID, "Error", err.Error())
		return "Unable to subscribe to the channel: " + err.Error(), http.StatusInternalServerError
	}

	p.GetMetrics().ObserveSubscription(metrics.SubscriptionConnected)
	if err = p.store.StoreChannelLink(&channelLink); err != nil {
		p.API.LogError("Error occurred while storing the channel link.", "Error", err.Error())
		return "Unable to create new link.", http.StatusInternalServerError
	}

	tx, err := p.store.BeginTx()
	if err != nil {
		p.API.LogError("Unable to begin the database transaction", "error", err.Error())
		return "Something went wrong", http.StatusInternalServerError
	}

	var txErr error
	defer func() {
		if txErr != nil {
			if err := p.store.RollbackTx(tx); err != nil {
				p.API.LogError("Unable to rollback database transaction", "error", err.Error())
			}
		}
	}()

	if txErr = p.store.SaveChannelSubscription(tx, storemodels.ChannelSubscription{
		SubscriptionID: channelsSubscription.ID,
		TeamID:         channelLink.MSTeamsTeamID,
		ChannelID:      channelLink.MSTeamsChannelID,
		ExpiresOn:      channelsSubscription.ExpiresOn,
		Secret:         p.getConfiguration().WebhookSecret,
	}); txErr != nil {
		p.API.LogError("Error occurred while saving the channel subscription.", "Error", txErr.Error())
		return "Error occurred while saving the subscription", http.StatusInternalServerError
	}

	if err := p.store.CommitTx(tx); err != nil {
		p.API.LogError("Unable to commit database transaction", "error", err.Error())
		return "Something went wrong", http.StatusInternalServerError
	}

	return "", http.StatusOK
}

func (p *Plugin) UnlinkChannels(userID, mattermostChannelID string) (string, int) {
	channel, appErr := p.API.GetChannel(mattermostChannelID)
	if appErr != nil {
		p.API.LogError("Unable to get the current channel details.", "ChannelID", mattermostChannelID, "error", appErr.Message)
		return "Unable to get the current channel details.", http.StatusInternalServerError
	}

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		p.API.LogError("Linking/unlinking a direct or group message is not allowed")
		return "Linking/unlinking a direct or group message is not allowed", http.StatusBadRequest
	}

	if !p.API.HasPermissionToChannel(userID, mattermostChannelID, model.PermissionManageChannelRoles) {
		p.API.LogError("Unable to unlink the channel, you have to be atleast a channel admin to unlink it.", "ChannelID", mattermostChannelID)
		return "Unable to unlink the channel, you have to be atleast a channel admin to unlink it.", http.StatusForbidden
	}

	link, err := p.store.GetLinkByChannelID(channel.Id)
	if err != nil {
		p.API.LogDebug("This Mattermost channel is not linked to any MS Teams channel.", "ChannelID", channel.Id, "error", err.Error())
		return "This Mattermost channel is not linked to any MS Teams channel.", http.StatusBadRequest
	}

	if err = p.store.DeleteLinkByChannelID(channel.Id); err != nil {
		p.API.LogDebug("Unable to delete the link by channel ID", "error", err.Error())
		return "Unable to delete link.", http.StatusInternalServerError
	}

	subscription, err := p.store.GetChannelSubscriptionByTeamsChannelID(link.MSTeamsChannelID)
	if err != nil {
		p.API.LogDebug("Unable to get the subscription by MS Teams channel ID", "error", err.Error())
		return "", http.StatusOK
	}

	if err = p.store.DeleteSubscription(subscription.SubscriptionID); err != nil {
		p.API.LogDebug("Unable to delete the subscription from the DB", "subscriptionID", subscription.SubscriptionID, "error", err.Error())
		return "", http.StatusOK
	}

	if err = p.msteamsAppClient.DeleteSubscription(subscription.SubscriptionID); err != nil {
		p.API.LogDebug("Unable to delete the subscription on MS Teams", "subscriptionID", subscription.SubscriptionID, "error", err.Error())
		return "", http.StatusOK
	}

	return "", http.StatusOK
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
