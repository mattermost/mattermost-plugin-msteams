package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-api/experimental/command"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

const msteamsCommand = "msteams-sync"
const commandWaitingMessage = "Please wait while your request is being processed."

func (p *Plugin) createMsteamsSyncCommand() *model.Command {
	iconData, err := command.GetIconData(p.API, "assets/msteams-sync-icon.svg")
	if err != nil {
		p.API.LogError("Unable to get the MS Teams icon for the slash command")
	}

	return &model.Command{
		Trigger:              msteamsCommand,
		AutoComplete:         true,
		AutoCompleteDesc:     "Manage synced channels between MS Teams and Mattermost",
		AutoCompleteHint:     "[command]",
		Username:             botUsername,
		DisplayName:          botDisplayName,
		AutocompleteData:     getAutocompleteData(),
		AutocompleteIconData: iconData,
	}
}

func (p *Plugin) cmdError(userID string, channelID string, detailedError string) (*model.CommandResponse, *model.AppError) {
	p.API.SendEphemeralPost(userID, &model.Post{
		Message:   detailedError,
		UserId:    p.userID,
		ChannelId: channelID,
	})
	return &model.CommandResponse{}, nil
}

func (p *Plugin) sendBotEphemeralPost(userID, channelID, message string) {
	_ = p.API.SendEphemeralPost(userID, &model.Post{
		Message:   message,
		UserId:    p.userID,
		ChannelId: channelID,
	})
}

func getAutocompleteData() *model.AutocompleteData {
	cmd := model.NewAutocompleteData(msteamsCommand, "[command]", "Manage MS Teams linked channels")

	link := model.NewAutocompleteData("link", "[msteams-team-id] [msteams-channel-id]", "Link current channel to a MS Teams channel")
	link.AddDynamicListArgument("[msteams-team-id]", getAutocompletePath("teams"), true)
	link.AddDynamicListArgument("[msteams-channel-id]", getAutocompletePath("channels"), true)
	cmd.AddCommand(link)

	unlink := model.NewAutocompleteData("unlink", "", "Unlink the current channel from the MS Teams channel")
	cmd.AddCommand(unlink)

	show := model.NewAutocompleteData("show", "", "Show MS Teams linked channel")
	cmd.AddCommand(show)

	showLinks := model.NewAutocompleteData("show-links", "", "Show all MS Teams linked channels")
	showLinks.RoleID = model.SystemAdminRoleId
	cmd.AddCommand(showLinks)

	connect := model.NewAutocompleteData("connect", "", "Connect your Mattermost account to your MS Teams account")
	cmd.AddCommand(connect)

	disconnect := model.NewAutocompleteData("disconnect", "", "Disconnect your Mattermost account from your MS Teams account")
	cmd.AddCommand(disconnect)

	connectBot := model.NewAutocompleteData("connect-bot", "", "Connect the bot account (only system admins can do this)")
	connectBot.RoleID = model.SystemAdminRoleId
	cmd.AddCommand(connectBot)

	disconnectBot := model.NewAutocompleteData("disconnect-bot", "", "Disconnect the bot account (only system admins can do this)")
	disconnectBot.RoleID = model.SystemAdminRoleId
	cmd.AddCommand(disconnectBot)

	promoteUser := model.NewAutocompleteData("promote", "", "Promote a user from synthetic user account to regular mattermost account")
	promoteUser.AddTextArgument("Username of the existing mattermost user", "username", `^[a-z0-9\.\-_:]+$`)
	promoteUser.AddTextArgument("The new username after the user is promoted", "new username", `^[a-z0-9\.\-_:]+$`)
	promoteUser.RoleID = model.SystemAdminRoleId
	cmd.AddCommand(promoteUser)

	return cmd
}

func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	split := strings.Fields(args.Command)
	command := split[0]
	var parameters []string
	action := ""
	if len(split) > 1 {
		action = split[1]
	}
	if len(split) > 2 {
		parameters = split[2:]
	}

	if command != "/"+msteamsCommand {
		return &model.CommandResponse{}, nil
	}

	if action == "link" {
		return p.executeLinkCommand(args, parameters)
	}

	if action == "unlink" {
		return p.executeUnlinkCommand(args)
	}

	if action == "show" {
		return p.executeShowCommand(args)
	}

	if action == "show-links" {
		return p.executeShowLinksCommand(args)
	}

	if action == "connect" {
		return p.executeConnectCommand(args)
	}

	if action == "connect-bot" {
		return p.executeConnectBotCommand(args)
	}

	if action == "disconnect" {
		return p.executeDisconnectCommand(args)
	}

	if action == "disconnect-bot" {
		return p.executeDisconnectBotCommand(args)
	}

	if action == "promote" {
		return p.executePromoteUserCommand(args, parameters)
	}

	return p.cmdError(args.UserId, args.ChannelId, "Unknown command. Valid options: link, unlink and show.")
}

func (p *Plugin) executeLinkCommand(args *model.CommandArgs, parameters []string) (*model.CommandResponse, *model.AppError) {
	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to get the current channel information.")
	}

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		return p.cmdError(args.UserId, args.ChannelId, "Linking/unlinking a direct or group message is not allowed")
	}

	canLinkChannel := p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManageChannelRoles)
	if !canLinkChannel {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to link the channel. You have to be a channel admin to link it.")
	}

	if len(parameters) < 2 {
		return p.cmdError(args.UserId, args.ChannelId, "Invalid link command, please pass the MS Teams team id and channel id as parameters.")
	}

	if !p.store.CheckEnabledTeamByTeamID(args.TeamId) {
		return p.cmdError(args.UserId, args.ChannelId, "This team is not enabled for MS Teams sync.")
	}

	link, err := p.store.GetLinkByChannelID(args.ChannelId)
	if err == nil && link != nil {
		return p.cmdError(args.UserId, args.ChannelId, "A link for this channel already exists. Please unlink the channel before you link again with another channel.")
	}

	link, err = p.store.GetLinkByMSTeamsChannelID(parameters[0], parameters[1])
	if err == nil && link != nil {
		return p.cmdError(args.UserId, args.ChannelId, "A link for this channel already exists. Please unlink the channel before you link again with another channel.")
	}

	client, err := p.GetClientForUser(args.UserId)
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to link the channel, looks like your account is not connected to MS Teams")
	}

	if _, err = client.GetChannelInTeam(parameters[0], parameters[1]); err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "MS Teams channel not found or you don't have the permissions to access it.")
	}

	channelLink := storemodels.ChannelLink{
		MattermostTeamID:    channel.TeamId,
		MattermostChannelID: channel.Id,
		MSTeamsTeamID:       parameters[0],
		MSTeamsChannelID:    parameters[1],
		Creator:             args.UserId,
	}

	channelsSubscription, err := p.msteamsAppClient.SubscribeToChannel(channelLink.MSTeamsTeamID, channelLink.MSTeamsChannelID, p.GetURL()+"/", p.getConfiguration().WebhookSecret)
	if err != nil {
		p.API.LogDebug("Unable to subscribe to the channel", "channelID", channelLink.MattermostChannelID, "error", err.Error())
		return p.cmdError(args.UserId, args.ChannelId, "Unable to subscribe to the channel")
	}

	if err = p.store.StoreChannelLink(&channelLink); err != nil {
		p.API.LogDebug("Unable to create the new link", "error", err.Error())
		return p.cmdError(args.UserId, args.ChannelId, "Unable to create new link.")
	}

	err = p.store.SaveChannelSubscription(storemodels.ChannelSubscription{
		SubscriptionID: channelsSubscription.ID,
		TeamID:         channelLink.MSTeamsTeamID,
		ChannelID:      channelLink.MSTeamsChannelID,
		ExpiresOn:      channelsSubscription.ExpiresOn,
		Secret:         p.getConfiguration().WebhookSecret,
	})
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to save the subscription in the monitoring system: "+err.Error())
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "The MS Teams channel is now linked to this Mattermost channel.")
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeUnlinkCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to get the current channel information.")
	}

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		return p.cmdError(args.UserId, args.ChannelId, "Linking/unlinking a direct or group message is not allowed")
	}

	canLinkChannel := p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManageChannelRoles)
	if !canLinkChannel {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to unlink the channel, you have to be a channel admin to unlink it.")
	}

	link, err := p.store.GetLinkByChannelID(channel.Id)
	if err != nil {
		p.API.LogDebug("Unable to get the link by channel ID", "error", err.Error())
		return p.cmdError(args.UserId, args.ChannelId, "This Mattermost channel is not linked to any MS Teams channel.")
	}

	if err = p.store.DeleteLinkByChannelID(channel.Id); err != nil {
		p.API.LogDebug("Unable to delete the link by channel ID", "error", err.Error())
		return p.cmdError(args.UserId, args.ChannelId, "Unable to delete link.")
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "The MS Teams channel is no longer linked to this Mattermost channel.")

	subscription, err := p.store.GetChannelSubscriptionByTeamsChannelID(link.MSTeamsChannelID)
	if err != nil {
		p.API.LogDebug("Unable to get the subscription by MS Teams channel ID", "error", err.Error())
		return &model.CommandResponse{}, nil
	}

	if err = p.store.DeleteSubscription(subscription.SubscriptionID); err != nil {
		p.API.LogDebug("Unable to delete the subscription from the DB", "subscriptionID", subscription.SubscriptionID, "error", err.Error())
		return &model.CommandResponse{}, nil
	}

	if err = p.msteamsAppClient.DeleteSubscription(subscription.SubscriptionID); err != nil {
		p.API.LogDebug("Unable to delete the subscription on MS Teams", "subscriptionID", subscription.SubscriptionID, "error", err.Error())
	}

	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeShowCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	link, err := p.store.GetLinkByChannelID(args.ChannelId)
	if err != nil || link == nil {
		return p.cmdError(args.UserId, args.ChannelId, "Link doesn't exist.")
	}

	msteamsTeam, err := p.msteamsAppClient.GetTeam(link.MSTeamsTeamID)
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to get the MS Teams team information.")
	}

	msteamsChannel, err := p.msteamsAppClient.GetChannelInTeam(link.MSTeamsTeamID, link.MSTeamsChannelID)
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to get the MS Teams channel information.")
	}

	text := fmt.Sprintf(
		"This channel is linked to the MS Teams Channel \"%s\" (with id: %s) in the Team \"%s\" (with the id: %s).",
		msteamsChannel.DisplayName,
		msteamsChannel.ID,
		msteamsTeam.DisplayName,
		msteamsTeam.ID,
	)

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, text)
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeShowLinksCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.API.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to execute the command, only system admins have access to execute this command.")
	}

	links, err := p.store.ListChannelLinksWithNames()
	if err != nil {
		p.API.LogDebug("Unable to get links from store", "Error", err.Error())
		return p.cmdError(args.UserId, args.ChannelId, "Something went wrong.")
	}

	if len(links) == 0 {
		return p.cmdError(args.UserId, args.ChannelId, "No links present.")
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, commandWaitingMessage)
	go p.SendLinksWithDetails(args.UserId, args.ChannelId, links)
	return &model.CommandResponse{}, nil
}

func (p *Plugin) SendLinksWithDetails(userID, channelID string, links []*storemodels.ChannelLink) {
	var sb strings.Builder
	sb.WriteString("| Mattermost Team | Mattermost Channel | MS Teams Team | MS Teams Channel | \n| :------|:--------|:-------|:-----------|")

	msTeamsTeamIDsVsNames, msTeamsChannelIDsVsNames, errorsFound := p.GetMSTeamsTeamAndChannelDetailsFromChannelLinks(links, userID, false)

	for _, link := range links {
		row := fmt.Sprintf(
			"\n|%s|%s|%s|%s|",
			link.MattermostTeamName,
			link.MattermostChannelName,
			msTeamsTeamIDsVsNames[link.MSTeamsTeamID],
			msTeamsChannelIDsVsNames[link.MSTeamsChannelID],
		)

		if row != "\n|||||" {
			sb.WriteString(row)
		}
	}

	if errorsFound {
		sb.WriteString("\nThere were some errors while fetching information. Please check the server logs.")
	}

	p.sendBotEphemeralPost(userID, channelID, sb.String())
}

func (p *Plugin) executeConnectCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	state := fmt.Sprintf("%s_%s", model.NewId(), args.UserId)
	if err := p.store.StoreOAuth2State(state); err != nil {
		p.API.LogError("Error in storing the OAuth state", "error", err.Error())
		return p.cmdError(args.UserId, args.ChannelId, "Error trying to connect the account, please try again.")
	}

	codeVerifier := model.NewId()
	if appErr := p.API.KVSet("_code_verifier_"+args.UserId, []byte(codeVerifier)); appErr != nil {
		p.API.LogError("Error in storing the code verifier", "error", appErr.Error())
		return p.cmdError(args.UserId, args.ChannelId, "Error trying to connect the account, please try again.")
	}

	connectURL := msteams.GetAuthURL(p.GetURL()+"/oauth-redirect", p.configuration.TenantID, p.configuration.ClientID, p.configuration.ClientSecret, state, codeVerifier)
	p.sendBotEphemeralPost(args.UserId, args.ChannelId, fmt.Sprintf("[Click here to connect your account](%s)", connectURL))
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeConnectBotCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.API.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to connect the bot account, only system admins can connect the bot account.")
	}

	state := fmt.Sprintf("%s_%s", model.NewId(), p.userID)
	if err := p.store.StoreOAuth2State(state); err != nil {
		p.API.LogError("Error in storing the OAuth state", "error", err.Error())
		return p.cmdError(args.UserId, args.ChannelId, "Error trying to connect the bot account, please try again.")
	}

	codeVerifier := model.NewId()
	appErr := p.API.KVSet("_code_verifier_"+p.GetBotUserID(), []byte(codeVerifier))
	if appErr != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Error trying to connect the bot account, please try again.")
	}

	connectURL := msteams.GetAuthURL(p.GetURL()+"/oauth-redirect", p.configuration.TenantID, p.configuration.ClientID, p.configuration.ClientSecret, state, codeVerifier)

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, fmt.Sprintf("[Click here to connect the bot account](%s)", connectURL))
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeDisconnectCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	teamsUserID, err := p.store.MattermostToTeamsUserID(args.UserId)
	if err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: the account is not connected")
		return &model.CommandResponse{}, nil
	}

	if _, err = p.store.GetTokenForMattermostUser(args.UserId); err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: the account is not connected")
		return &model.CommandResponse{}, nil
	}

	err = p.store.SetUserInfo(args.UserId, teamsUserID, nil)
	if err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, fmt.Sprintf("Error: unable to disconnect your account, %s", err.Error()))
		return &model.CommandResponse{}, nil
	}

	p.API.PublishWebSocketEvent(
		"disconnect",
		nil,
		&model.WebsocketBroadcast{UserId: args.UserId},
	)

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Your account has been disconnected.")
	if err := p.store.DeleteDMAndGMChannelPromptTime(args.UserId); err != nil {
		p.API.LogDebug("Unable to delete the last prompt timestamp for the user", "MMUserID", args.UserId, "Error", err.Error())
	}

	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeDisconnectBotCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.API.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to disconnect the bot account, only system admins can disconnect the bot account.")
	}

	if _, err := p.store.MattermostToTeamsUserID(p.userID); err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: unable to find the connected bot account")
		return &model.CommandResponse{}, nil
	}

	if err := p.store.DeleteUserInfo(p.userID); err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, fmt.Sprintf("Error: unable to disconnect the bot account, %s", err.Error()))
		return &model.CommandResponse{}, nil
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "The bot account has been disconnected.")
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executePromoteUserCommand(args *model.CommandArgs, parameters []string) (*model.CommandResponse, *model.AppError) {
	if len(parameters) != 2 {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Invalid promote command, please pass the current username and promoted username as parameters.")
		return &model.CommandResponse{}, nil
	}

	if !p.API.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Unable to execute the command, only system admins have access to execute this command.")
		return &model.CommandResponse{}, nil
	}

	username := parameters[0]
	newUsername := parameters[1]

	var user *model.User
	var appErr *model.AppError
	if strings.HasPrefix(username, "@") {
		user, appErr = p.API.GetUserByUsername(username[1:])
	} else {
		user, appErr = p.API.GetUserByUsername(username)
	}
	if appErr != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: Unable to promote account "+username+", user not found")
		return &model.CommandResponse{}, nil
	}

	userID, err := p.store.MattermostToTeamsUserID(user.Id)
	if err != nil || userID == "" {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: Unable to promote account "+username+", it is not a known msteams user account")
		return &model.CommandResponse{}, nil
	}

	if user.RemoteId == nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: Unable to promote account "+username+", it is already a regular account")
		return &model.CommandResponse{}, nil
	}

	newUser, appErr := p.API.GetUserByUsername(newUsername)
	if appErr == nil && newUser != nil && newUser.Id != user.Id {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: the promoted username already exists, please use a different username.")
		return &model.CommandResponse{}, nil
	}

	user.RemoteId = nil
	user.Username = newUsername
	_, appErr = p.API.UpdateUser(user)
	if appErr != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: Unable to promote account "+username)
		return &model.CommandResponse{}, nil
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Account "+username+" has been promoted and updated the username to "+newUsername)
	return &model.CommandResponse{}, nil
}

func getAutocompletePath(path string) string {
	return "plugins/" + pluginID + "/autocomplete/" + path
}
