package main

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/command"
)

const msteamsCommand = "msteams"
const commandWaitingMessage = "Please wait while your request is being processed."

func (p *Plugin) createCommand(syncLinkedChannels bool) *model.Command {
	iconData, err := command.GetIconData(p.API, "assets/icon.svg")
	if err != nil {
		p.API.LogWarn("Unable to get the MS Teams icon for the slash command")
	}

	autoCompleteData := getAutocompleteData(syncLinkedChannels)
	p.subCommandsMutex.Lock()
	defer p.subCommandsMutex.Unlock()
	p.subCommands = make([]string, 0, len(autoCompleteData.SubCommands))
	for i := range autoCompleteData.SubCommands {
		p.subCommands = append(p.subCommands, autoCompleteData.SubCommands[i].Trigger)
	}

	return &model.Command{
		Trigger:              msteamsCommand,
		AutoComplete:         true,
		AutoCompleteDesc:     "Manage synced channels between MS Teams and Mattermost",
		AutoCompleteHint:     "[command]",
		Username:             botUsername,
		DisplayName:          botDisplayName,
		AutocompleteData:     autoCompleteData,
		AutocompleteIconData: iconData,
	}
}

func (p *Plugin) cmdSuccess(args *model.CommandArgs, text string) (*model.CommandResponse, *model.AppError) {
	// Delegate to an ephemeral post from the bot, since we can't customize the sender here.
	p.sendBotEphemeralPost(args.UserId, args.ChannelId, text)

	return &model.CommandResponse{}, nil
}

func (p *Plugin) cmdError(args *model.CommandArgs, detailedError string) (*model.CommandResponse, *model.AppError) {
	// Delegate to an ephemeral post from the bot, since we can't customize the sender here.
	p.sendBotEphemeralPost(args.UserId, args.ChannelId, detailedError)

	return &model.CommandResponse{}, nil
}

func (p *Plugin) sendBotEphemeralPost(userID, channelID, message string) {
	_ = p.API.SendEphemeralPost(userID, &model.Post{
		Message:   message,
		UserId:    p.botUserID,
		ChannelId: channelID,
	})
}

func getAutocompleteData(syncLinkedChannels bool) *model.AutocompleteData {
	cmd := model.NewAutocompleteData(msteamsCommand, "[command]", "Manage MS Teams linked channels")

	if syncLinkedChannels {
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
	}

	connect := model.NewAutocompleteData("connect", "", "Connect your Mattermost account to your MS Teams account")
	cmd.AddCommand(connect)

	disconnect := model.NewAutocompleteData("disconnect", "", "Disconnect your Mattermost account from your MS Teams account")
	cmd.AddCommand(disconnect)

	status := model.NewAutocompleteData("status", "", "Show your connection status")
	cmd.AddCommand(status)

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

	notifications := model.NewAutocompleteData("notifications", "", "Enable or disable notifications from MSTeams. You must be connected to perform this action.")
	notifications.AddStaticListArgument("status", true, []model.AutocompleteListItem{
		{Item: "status", HelpText: "Show current notification status."},
		{Item: "on", HelpText: "Enable notifications from chats and group chats."},
		{Item: "off", HelpText: "Disable notifications from chats and group chats."},
	})
	cmd.AddCommand(notifications)

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

	if p.getConfiguration().SyncLinkedChannels {
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

	if action == "status" {
		return p.executeStatusCommand(args)
	}

	if action == "notifications" {
		return p.executeNotificationsCommand(args, parameters)
	}

	p.subCommandsMutex.RLock()
	list := strings.Join(p.subCommands, ", ")
	p.subCommandsMutex.RUnlock()
	return p.cmdError(args, "Unknown command. Valid options: "+list)
}

func (p *Plugin) executeLinkCommand(args *model.CommandArgs, parameters []string) (*model.CommandResponse, *model.AppError) {
	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return p.cmdError(args, "Unable to get the current channel information.")
	}

	if channel.IsGroupOrDirect() {
		return p.cmdError(args, "Linking/unlinking a direct or group message is not allowed")
	}

	canLinkChannel := p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManageChannelRoles)
	if !canLinkChannel {
		return p.cmdError(args, "Unable to link the channel. You have to be a channel admin to link it.")
	}

	if len(parameters) < 2 {
		return p.cmdError(args, "Invalid link command, please pass the MS Teams team id and channel id as parameters.")
	}

	link, err := p.store.GetLinkByChannelID(args.ChannelId)
	if err == nil && link != nil {
		return p.cmdError(args, "A link for this channel already exists. Please unlink the channel before you link again with another channel.")
	}

	link, err = p.store.GetLinkByMSTeamsChannelID(parameters[0], parameters[1])
	if err == nil && link != nil {
		return p.cmdError(args, "A link for this channel already exists. Please unlink the channel before you link again with another channel.")
	}

	client, err := p.GetClientForUser(args.UserId)
	if err != nil {
		return p.cmdError(args, "Unable to link the channel, looks like your account is not connected to MS Teams")
	}

	if _, err = client.GetChannelInTeam(parameters[0], parameters[1]); err != nil {
		return p.cmdError(args, "MS Teams channel not found or you don't have the permissions to access it.")
	}

	channelLink := storemodels.ChannelLink{
		MattermostTeamID:    channel.TeamId,
		MattermostChannelID: channel.Id,
		MSTeamsTeam:         parameters[0],
		MSTeamsChannel:      parameters[1],
		Creator:             args.UserId,
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, commandWaitingMessage)
	channelsSubscription, err := p.GetClientForApp().SubscribeToChannel(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, p.GetURL()+"/", p.getConfiguration().WebhookSecret, p.getBase64Certificate())
	if err != nil {
		p.API.LogWarn("Unable to subscribe to the channel", "channel_id", channelLink.MattermostChannelID, "error", err.Error())
		return p.cmdError(args, "Unable to subscribe to the channel")
	}

	p.GetMetrics().ObserveSubscription(metrics.SubscriptionConnected)
	if err = p.store.StoreChannelLink(&channelLink); err != nil {
		p.API.LogWarn("Unable to create the new link", "error", err.Error())
		return p.cmdError(args, "Unable to create new link.")
	}

	if err = p.store.SaveChannelSubscription(storemodels.ChannelSubscription{
		SubscriptionID: channelsSubscription.ID,
		TeamID:         channelLink.MSTeamsTeam,
		ChannelID:      channelLink.MSTeamsChannel,
		ExpiresOn:      channelsSubscription.ExpiresOn,
		Secret:         p.getConfiguration().WebhookSecret,
	}); err != nil {
		p.API.LogWarn("Unable to save the subscription in the DB", "error", err.Error())
		return p.cmdError(args, "Error occurred while saving the subscription")
	}

	if p.getConfiguration().UseSharedChannels {
		if _, err = p.API.ShareChannel(&model.SharedChannel{
			ChannelId: channelLink.MattermostChannelID,
			TeamId:    channelLink.MattermostTeamID,
			Home:      true,
			ReadOnly:  false,
			CreatorId: p.botUserID,
			RemoteId:  p.remoteID,
			ShareName: channelLink.MattermostChannelID,
		}); err != nil {
			p.API.LogWarn("Failed to share channel", "channel_id", channelLink.MattermostChannelID, "error", err.Error())
		} else {
			p.API.LogInfo("Shared channel", "channel_id", channelLink.MattermostChannelID)
		}
	}

	return p.cmdSuccess(args, "The MS Teams channel is now linked to this Mattermost channel.")
}

func (p *Plugin) executeUnlinkCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return p.cmdError(args, "Unable to get the current channel information.")
	}

	if channel.IsGroupOrDirect() {
		return p.cmdError(args, "Linking/unlinking a direct or group message is not allowed")
	}

	canLinkChannel := p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManageChannelRoles)
	if !canLinkChannel {
		return p.cmdError(args, "Unable to unlink the channel, you have to be a channel admin to unlink it.")
	}

	link, err := p.store.GetLinkByChannelID(channel.Id)
	if err != nil {
		p.API.LogWarn("Unable to get the link by channel ID", "error", err.Error())
		return p.cmdError(args, "This Mattermost channel is not linked to any MS Teams channel.")
	}

	if err = p.store.DeleteLinkByChannelID(channel.Id); err != nil {
		p.API.LogWarn("Unable to delete the link by channel ID", "error", err.Error())
		return p.cmdError(args, "Unable to delete link.")
	}

	subscription, err := p.store.GetChannelSubscriptionByTeamsChannelID(link.MSTeamsChannel)
	if err != nil {
		p.API.LogWarn("Unable to get the subscription by MS Teams channel ID", "error", err.Error())
	} else if subscription != nil {
		if err = p.store.DeleteSubscription(subscription.SubscriptionID); err != nil {
			p.API.LogWarn("Unable to delete the subscription from the DB", "subscription_id", subscription.SubscriptionID, "error", err.Error())
		}

		if p.getConfiguration().UseSharedChannels {
			if _, err = p.API.UnshareChannel(link.MattermostChannelID); err != nil {
				p.API.LogWarn("Failed to unshare channel", "channel_id", link.MattermostChannelID, "subscription_id", subscription.SubscriptionID, "error", err.Error())
			} else {
				p.API.LogInfo("Unshared channel", "channel_id", link.MattermostChannelID)
			}
		}

		if err = p.GetClientForApp().DeleteSubscription(subscription.SubscriptionID); err != nil {
			p.API.LogWarn("Unable to delete the subscription on MS Teams", "subscription_id", subscription.SubscriptionID, "error", err.Error())
		}
	}

	return p.cmdSuccess(args, "The MS Teams channel is no longer linked to this Mattermost channel.")
}

func (p *Plugin) executeShowCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	link, err := p.store.GetLinkByChannelID(args.ChannelId)
	if err != nil || link == nil {
		return p.cmdError(args, "Link doesn't exist.")
	}

	msteamsTeam, err := p.GetClientForApp().GetTeam(link.MSTeamsTeam)
	if err != nil {
		return p.cmdError(args, "Unable to get the MS Teams team information.")
	}

	msteamsChannel, err := p.GetClientForApp().GetChannelInTeam(link.MSTeamsTeam, link.MSTeamsChannel)
	if err != nil {
		return p.cmdError(args, "Unable to get the MS Teams channel information.")
	}

	text := fmt.Sprintf(
		"This channel is linked to the MS Teams Channel \"%s\" in the Team \"%s\".",
		msteamsChannel.DisplayName,
		msteamsTeam.DisplayName,
	)

	return p.cmdSuccess(args, text)
}

func (p *Plugin) executeShowLinksCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.API.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return p.cmdError(args, "Unable to execute the command, only system admins have access to execute this command.")
	}

	links, err := p.store.ListChannelLinksWithNames()
	if err != nil {
		p.API.LogWarn("Unable to get links from store", "error", err.Error())
		return p.cmdError(args, "Something went wrong.")
	}

	if len(links) == 0 {
		return p.cmdError(args, "No links present.")
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, commandWaitingMessage)
	go p.SendLinksWithDetails(args.UserId, args.ChannelId, links)
	return &model.CommandResponse{}, nil
}

func (p *Plugin) SendLinksWithDetails(userID, channelID string, links []*storemodels.ChannelLink) {
	defer func() {
		if r := recover(); r != nil {
			p.GetMetrics().ObserveGoroutineFailure()
			p.API.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	var sb strings.Builder
	sb.WriteString("| Mattermost Team | Mattermost Channel | MS Teams Team | MS Teams Channel |\n| :------|:--------|:-------|:-----------|")
	errorsFound := false
	msTeamsTeamIDsVsNames := make(map[string]string)
	msTeamsChannelIDsVsNames := make(map[string]string)
	msTeamsTeamIDsVsChannelsQuery := make(map[string]string)

	for _, link := range links {
		msTeamsTeamIDsVsNames[link.MSTeamsTeam] = ""
		msTeamsChannelIDsVsNames[link.MSTeamsChannel] = ""

		// Build the channels query for each team
		if msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeam] == "" {
			msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeam] = "id in ("
		} else if channelsQuery := msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeam]; channelsQuery[len(channelsQuery)-1:] != "(" {
			msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeam] += ","
		}

		msTeamsTeamIDsVsChannelsQuery[link.MSTeamsTeam] += "'" + link.MSTeamsChannel + "'"
	}

	// Get MS Teams display names for each unique team ID and store it
	teamDetailsErr := p.GetMSTeamsTeamDetails(msTeamsTeamIDsVsNames)
	errorsFound = errorsFound || teamDetailsErr

	// Get MS Teams channel details for all channels for each unique team
	channelDetailsErr := p.GetMSTeamsChannelDetailsForAllTeams(msTeamsTeamIDsVsChannelsQuery, msTeamsChannelIDsVsNames)
	errorsFound = errorsFound || channelDetailsErr

	for _, link := range links {
		row := fmt.Sprintf(
			"\n|%s|%s|%s|%s|",
			link.MattermostTeamName,
			link.MattermostChannelName,
			msTeamsTeamIDsVsNames[link.MSTeamsTeam],
			msTeamsChannelIDsVsNames[link.MSTeamsChannel],
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

func (p *Plugin) GetMSTeamsTeamDetails(msTeamsTeamIDsVsNames map[string]string) bool {
	var msTeamsFilterQuery strings.Builder
	msTeamsFilterQuery.WriteString("id in (")
	for teamID := range msTeamsTeamIDsVsNames {
		msTeamsFilterQuery.WriteString("'" + teamID + "', ")
	}

	teamsQuery := msTeamsFilterQuery.String()
	teamsQuery = strings.TrimSuffix(teamsQuery, ", ") + ")"
	msTeamsTeams, err := p.GetClientForApp().GetTeams(teamsQuery)
	if err != nil {
		p.API.LogWarn("Unable to get the MS Teams teams information", "error", err.Error())
		return true
	}

	for _, msTeamsTeam := range msTeamsTeams {
		msTeamsTeamIDsVsNames[msTeamsTeam.ID] = msTeamsTeam.DisplayName
	}

	return false
}

func (p *Plugin) GetMSTeamsChannelDetailsForAllTeams(msTeamsTeamIDsVsChannelsQuery, msTeamsChannelIDsVsNames map[string]string) bool {
	errorsFound := false
	for teamID, channelsQuery := range msTeamsTeamIDsVsChannelsQuery {
		channels, err := p.GetClientForApp().GetChannelsInTeam(teamID, channelsQuery+")")
		if err != nil {
			p.API.LogWarn("Unable to get the MS Teams channel information for the team", "team_id", teamID, "error", err.Error())
			errorsFound = true
		}

		for _, channel := range channels {
			msTeamsChannelIDsVsNames[channel.ID] = channel.DisplayName
		}
	}

	return errorsFound
}

func (p *Plugin) executeConnectCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if storedToken, _ := p.store.GetTokenForMattermostUser(args.UserId); storedToken != nil {
		return p.cmdError(args, "You are already connected to MS Teams. Please disconnect your account first before connecting again.")
	}

	genericErrorMessage := "Error in trying to connect the account, please try again."

	hasRightToConnect, err := p.UserHasRightToConnect(args.UserId)
	if err != nil {
		p.API.LogWarn("Error in checking if the user has the right to connect", "user_id", args.UserId, "error", err.Error())
		return p.cmdError(args, genericErrorMessage)
	}

	if !hasRightToConnect {
		canOpenlyConnect, nAvailable, openConnectErr := p.UserCanOpenlyConnect(args.UserId)
		if openConnectErr != nil {
			p.API.LogWarn("Error in checking if the user can openly connect", "user_id", args.UserId, "error", openConnectErr.Error())
			return p.cmdError(args, genericErrorMessage)
		}

		if !canOpenlyConnect {
			if nAvailable > 0 {
				// spots available, but need to be on whitelist in order to connect
				return p.cmdError(args, "You cannot connect your account at this time because an invitation is required. Please contact your system administrator to request an invitation.")
			}
			return p.cmdError(args, "You cannot connect your account because the maximum limit of users allowed to connect has been reached. Please contact your system administrator.")
		}
	}

	p.SendEphemeralConnectMessage(args.ChannelId, args.UserId, "")
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeConnectBotCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.API.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return p.cmdError(args, "Unable to connect the bot account, only system admins can connect the bot account.")
	}

	if storedToken, _ := p.store.GetTokenForMattermostUser(p.botUserID); storedToken != nil {
		return p.cmdError(args, "The bot account is already connected to MS Teams. Please disconnect the bot account first before connecting again.")
	}

	p.SendConnectBotMessage(args.ChannelId, args.UserId)
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeDisconnectCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	teamsUserID, err := p.store.MattermostToTeamsUserID(args.UserId)
	if err != nil {
		return p.cmdSuccess(args, "Error: the account is not connected")
	}

	if token, _ := p.store.GetTokenForMattermostUser(args.UserId); token == nil {
		return p.cmdSuccess(args, "Error: the account is not connected")
	}

	err = p.store.SetUserInfo(args.UserId, teamsUserID, nil)
	if err != nil {
		return p.cmdSuccess(args, fmt.Sprintf("Error: unable to disconnect your account, %s", err.Error()))
	}

	p.API.PublishWebSocketEvent(WSEventUserDisconnected, map[string]any{}, &model.WebsocketBroadcast{
		UserId: args.UserId,
	})

	err = p.setNotificationPreference(args.UserId, false)
	if err != nil {
		p.API.LogWarn("unable to disable notifications preference", "error", err.Error())
	}

	return p.cmdSuccess(args, "Your account has been disconnected.")
}

func (p *Plugin) executeDisconnectBotCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.API.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return p.cmdError(args, "Unable to disconnect the bot account, only system admins can disconnect the bot account.")
	}

	if _, err := p.store.MattermostToTeamsUserID(p.botUserID); err != nil {
		return p.cmdSuccess(args, "Error: unable to find the connected bot account")
	}

	if err := p.store.DeleteUserInfo(p.botUserID); err != nil {
		return p.cmdSuccess(args, fmt.Sprintf("Error: unable to disconnect the bot account, %s", err.Error()))
	}

	return p.cmdSuccess(args, "The bot account has been disconnected.")
}

func (p *Plugin) executePromoteUserCommand(args *model.CommandArgs, parameters []string) (*model.CommandResponse, *model.AppError) {
	if len(parameters) != 2 {
		return p.cmdSuccess(args, "Invalid promote command, please pass the current username and promoted username as parameters.")
	}

	if !p.API.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return p.cmdSuccess(args, "Unable to execute the command, only system admins have access to execute this command.")
	}

	username := strings.TrimPrefix(parameters[0], "@")
	newUsername := strings.TrimPrefix(parameters[1], "@")

	user, appErr := p.API.GetUserByUsername(username)
	if appErr != nil {
		return p.cmdSuccess(args, "Error: Unable to promote account "+username+", user not found")
	}

	userID, err := p.store.MattermostToTeamsUserID(user.Id)
	if err != nil || userID == "" {
		return p.cmdSuccess(args, "Error: Unable to promote account "+username+", it is not a known msteams user account")
	}

	if user.RemoteId == nil {
		return p.cmdSuccess(args, "Error: Unable to promote account "+username+", it is already a regular account")
	}

	newUser, appErr := p.API.GetUserByUsername(newUsername)
	if appErr == nil && newUser != nil && newUser.Id != user.Id {
		return p.cmdSuccess(args, "Error: the promoted username already exists, please use a different username.")
	}

	user.RemoteId = nil
	user.Username = newUsername
	_, appErr = p.API.UpdateUser(user)
	if appErr != nil {
		p.API.LogWarn("Unable to update the user during promotion", "user_id", user.Id, "error", appErr.Error())
		return p.cmdSuccess(args, "Error: Unable to promote account "+username)
	}

	return p.cmdSuccess(args, "Account "+username+" has been promoted and updated the username to "+newUsername)
}

func (p *Plugin) executeStatusCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if storedToken, err := p.store.GetTokenForMattermostUser(args.UserId); err != nil {
		// TODO: We will need to distinguish real errors from "row not found" later.
		return p.cmdSuccess(args, "Your account is not connected to Teams.")
	} else if storedToken != nil {
		return p.cmdSuccess(args, "Your account is connected to Teams.")
	}

	return p.cmdSuccess(args, "Your account is not connected to Teams.")
}

func (p *Plugin) executeNotificationsCommand(args *model.CommandArgs, parameters []string) (*model.CommandResponse, *model.AppError) {
	if len(parameters) != 1 {
		return p.cmdSuccess(args, "Invalid notifications command, one argument is required.")
	}

	isConnected, err := p.IsUserConnected(args.UserId)
	if err != nil {
		p.API.LogWarn("unable to check if the user is connected", "error", err.Error())
		return p.cmdError(args, "Error: Unable to get the connection status")
	}
	if !isConnected {
		return p.cmdSuccess(args, "Error: Your account is not connected to Teams. To use this feature, please connect your account with `/msteams connect`.")
	}

	notificationPreferenceEnabled := p.getNotificationPreference(args.UserId)
	switch strings.ToLower(parameters[0]) {
	case "status":
		status := "disabled"
		if notificationPreferenceEnabled {
			status = "enabled"
		}
		return p.cmdSuccess(args, fmt.Sprintf("Notifications from chats and group chats in MS Teams are currently %s.", status))
	case "on":
		if !notificationPreferenceEnabled {
			err = p.setNotificationPreference(args.UserId, true)
			if err != nil {
				p.API.LogWarn("unable to enable notifications", "error", err.Error())
				return p.cmdError(args, "Error: Unable to enable notifications.")
			}
		}
		return p.cmdSuccess(args, "Notifications from chats and group chats in MS Teams are now enabled.")
	case "off":
		if notificationPreferenceEnabled {
			err = p.setNotificationPreference(args.UserId, false)
			if err != nil {
				p.API.LogWarn("unable to disable notifications", "error", err.Error())
				return p.cmdError(args, "Error: Unable to disable notifications.")
			}
		}
		return p.cmdSuccess(args, "Notifications from chats and group chats in MS Teams are now disabled.")
	}

	return p.cmdSuccess(args, parameters[0]+" is not a valid argument.")
}

func getAutocompletePath(path string) string {
	return "plugins/" + pluginID + "/autocomplete/" + path
}
