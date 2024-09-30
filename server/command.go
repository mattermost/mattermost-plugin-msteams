package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/command"
)

const msteamsCommand = "msteams"

func (p *Plugin) createCommand() *model.Command {
	iconData, err := command.GetIconData(p.API, "assets/icon.svg")
	if err != nil {
		p.API.LogWarn("Unable to get the MS Teams icon for the slash command")
	}

	autoCompleteData := getAutocompleteData()
	p.subCommandsMutex.Lock()
	defer p.subCommandsMutex.Unlock()
	p.subCommands = make([]string, 0, len(autoCompleteData.SubCommands))
	for i := range autoCompleteData.SubCommands {
		p.subCommands = append(p.subCommands, autoCompleteData.SubCommands[i].Trigger)
	}

	return &model.Command{
		Trigger:              msteamsCommand,
		AutoComplete:         true,
		AutoCompleteDesc:     "Manage the MS Teams Integration with Mattermost",
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

func getAutocompleteData() *model.AutocompleteData {
	cmd := model.NewAutocompleteData(msteamsCommand, "[command]", "Manage MS Teams")

	connect := model.NewAutocompleteData("connect", "", "Connect your Mattermost account to your MS Teams account")
	cmd.AddCommand(connect)

	disconnect := model.NewAutocompleteData("disconnect", "", "Disconnect your Mattermost account from your MS Teams account")
	cmd.AddCommand(disconnect)

	status := model.NewAutocompleteData("status", "", "Show your connection status")
	cmd.AddCommand(status)

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

	if action == "connect" {
		return p.executeConnectCommand(args)
	}

	if action == "disconnect" {
		return p.executeDisconnectCommand(args)
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

	p.API.LogInfo("User disconnected from Teams", "user_id", args.UserId, "teams_user_id", teamsUserID)

	p.API.PublishWebSocketEvent(WSEventUserDisconnected, map[string]any{}, &model.WebsocketBroadcast{
		UserId: args.UserId,
	})

	err = p.setNotificationPreference(args.UserId, false)
	if err != nil {
		p.API.LogWarn("unable to disable notifications preference", "error", err.Error())
	}

	return p.cmdSuccess(args, "Your account has been disconnected.")
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
	command := ""
	if len(parameters) == 0 {
		command = "status"
	} else if len(parameters) == 1 {
		command = strings.ToLower(parameters[0])
	} else {
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
	switch command {
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

	return p.cmdSuccess(args, command+" is not a valid argument.")
}
