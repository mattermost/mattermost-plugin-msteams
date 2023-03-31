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

func (p *Plugin) createMsteamsSyncCommand() *model.Command {
	iconData, err := command.GetIconData(p.API, "assets/msteams-sync-icon.svg")
	if err != nil {
		p.API.LogError("Unable to get the msteams icon for the slash command")
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
	p.API.SendEphemeralPost(userID, &model.Post{
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

	return cmd
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
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
		return p.executeLinkCommand(c, args, parameters)
	}

	if action == "unlink" {
		return p.executeUnlinkCommand(c, args)
	}

	if action == "show" {
		return p.executeShowCommand(c, args)
	}

	if action == "connect" {
		return p.executeConnectCommand(c, args)
	}

	if action == "connect-bot" {
		return p.executeConnectBotCommand(c, args)
	}

	if action == "disconnect" {
		return p.executeDisconnectCommand(c, args)
	}

	if action == "disconnect-bot" {
		return p.executeDisconnectBotCommand(c, args)
	}

	return p.cmdError(args.UserId, args.ChannelId, "Unknown command. Valid options: link, unlink and show.")
}

func (p *Plugin) executeLinkCommand(c *plugin.Context, args *model.CommandArgs, parameters []string) (*model.CommandResponse, *model.AppError) {
	if !p.IsSystemAdmin(args.UserId) && !p.IsChannelAdmin(args.UserId, args.ChannelId) {
		return p.cmdError(args.UserId, args.ChannelId, "This command can be run only by channel and system admins.")
	}

	if len(parameters) < 2 {
		return p.cmdError(args.UserId, args.ChannelId, "Invalid link command, please pass the MS Teams team id and channel id as parameters.")
	}

	if !p.store.CheckEnabledTeamByTeamID(args.TeamId) {
		return p.cmdError(args.UserId, args.ChannelId, "This team is not enabled for MS Teams sync.")
	}

	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to get the current channel information.")
	}

	canLinkChannel := channel.Type == model.ChannelTypeOpen && p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManagePublicChannelProperties)
	canLinkChannel = canLinkChannel || (channel.Type == model.ChannelTypePrivate && p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManagePrivateChannelProperties))
	if !canLinkChannel {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to link the channel. You have to be a channel admin to link it.")
	}

	link, err := p.store.GetLinkByChannelID(args.ChannelId)
	if err == nil && link != nil {
		return p.cmdError(args.UserId, args.ChannelId, "A link for this channel already exists, please remove unlink the channel before you link a new one")
	}

	link, err = p.store.GetLinkByMSTeamsChannelID(parameters[0], parameters[1])
	if err == nil && link != nil {
		return p.cmdError(args.UserId, args.ChannelId, "A link to this MS Teams channel already exists, please remove unlink the channel before you link a new one")
	}

	client, err := p.GetClientForUser(args.UserId)
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to link the channel, looks like your account is not connected to MS Teams")
	}

	_, err = client.GetChannel(parameters[0], parameters[1])
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "MS Teams channel not found or you don't have the permissions to access it.")
	}

	err = p.store.StoreChannelLink(&storemodels.ChannelLink{
		MattermostTeam:    channel.TeamId,
		MattermostChannel: channel.Id,
		MSTeamsTeam:       parameters[0],
		MSTeamsChannel:    parameters[1],
	})
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to create new link.")
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "The MS Teams channel is now linked to this Mattermost channel")
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeUnlinkCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.IsSystemAdmin(args.UserId) && !p.IsChannelAdmin(args.UserId, args.ChannelId) {
		return p.cmdError(args.UserId, args.ChannelId, "This command can be run only by channel and system admins.")
	}

	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to get the current channel information.")
	}

	canLinkChannel := channel.Type == model.ChannelTypeOpen && p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManagePublicChannelProperties)
	canLinkChannel = canLinkChannel || (channel.Type == model.ChannelTypePrivate && p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManagePrivateChannelProperties))
	if !canLinkChannel {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to unlink the channel, you has to be a channel admin to unlink it.")
	}

	err := p.store.DeleteLinkByChannelID(channel.Id)
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to delete link.")
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "The MS Teams channel is no longer linked to this Mattermost channel")
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeShowCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	link, err := p.store.GetLinkByChannelID(args.ChannelId)
	if err != nil || link == nil {
		return p.cmdError(args.UserId, args.ChannelId, "Link doesn't exists.")
	}

	msteamsTeam, err := p.msteamsAppClient.GetTeam(link.MSTeamsTeam)
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to get the MS Teams team information.")
	}

	msteamsChannel, err := p.msteamsAppClient.GetChannel(link.MSTeamsTeam, link.MSTeamsChannel)
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

func (p *Plugin) executeConnectCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	messageChan := make(chan string)
	go func(userID string, messageChan chan string) {
		tokenSource, err := msteams.RequestUserToken(p.configuration.TenantID, p.configuration.ClientID, messageChan)
		if err != nil {
			messageChan <- fmt.Sprintf("Error: unable to link your account, %s", err.Error())
			return
		}

		token, err := tokenSource.Token()
		if err != nil {
			messageChan <- fmt.Sprintf("Error: unable to link your account, %s", err.Error())
			return
		}

		client := msteams.NewTokenClient(p.configuration.TenantID, p.configuration.ClientID, token, p.API.LogError)
		if err = client.Connect(); err != nil {
			messageChan <- fmt.Sprintf("Error: unable to link your account, %s", err.Error())
			return
		}

		msteamsUserID, err := client.GetMyID()
		if err != nil {
			messageChan <- fmt.Sprintf("Error: unable to link your account, %s", err.Error())
			return
		}

		err = p.store.SetUserInfo(userID, msteamsUserID, token)
		if err != nil {
			messageChan <- fmt.Sprintf("Error: unable to link your account, %s", err.Error())
			return
		}

		messageChan <- "Your account has been connected"
	}(args.UserId, messageChan)

	message := <-messageChan
	p.sendBotEphemeralPost(args.UserId, args.ChannelId, message)
	message = <-messageChan
	p.sendBotEphemeralPost(args.UserId, args.ChannelId, message)
	close(messageChan)
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeConnectBotCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.API.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to connect the bot account, only system admins can connect the bot account.")
	}

	messageChan := make(chan string)
	go func(userID string, messageChan chan string) {
		tokenSource, err := msteams.RequestUserToken(p.configuration.TenantID, p.configuration.ClientID, messageChan)
		if err != nil {
			messageChan <- fmt.Sprintf("Error: unable to link the bot account, %s", err.Error())
			return
		}

		token, err := tokenSource.Token()
		if err != nil {
			messageChan <- fmt.Sprintf("Error: unable to link the bot account, %s", err.Error())
			return
		}

		client := msteams.NewTokenClient(p.configuration.TenantID, p.configuration.ClientID, token, p.API.LogError)
		if err = client.Connect(); err != nil {
			messageChan <- fmt.Sprintf("Error: unable to link the bot account, %s", err.Error())
			return
		}

		msteamsUserID, err := client.GetMyID()
		if err != nil {
			messageChan <- fmt.Sprintf("Error: unable to link the bot account, %s", err.Error())
			return
		}

		err = p.store.SetUserInfo(userID, msteamsUserID, token)
		if err != nil {
			messageChan <- fmt.Sprintf("Error: unable to link the bot account, %s", err.Error())
			return
		}

		messageChan <- "The bot account has been connected"
	}(p.userID, messageChan)

	message := <-messageChan
	p.sendBotEphemeralPost(args.UserId, args.ChannelId, message)
	message = <-messageChan
	p.sendBotEphemeralPost(args.UserId, args.ChannelId, message)
	close(messageChan)
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeDisconnectCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	teamsUserID, err := p.store.MattermostToTeamsUserID(args.UserId)
	if err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: the account is not connected")
		return &model.CommandResponse{}, nil
	}

	err = p.store.SetUserInfo(args.UserId, teamsUserID, nil)
	if err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, fmt.Sprintf("Error: unable to disconnect your account, %s", err.Error()))
		return &model.CommandResponse{}, nil
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Your account has been disconnected")

	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeDisconnectBotCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.API.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to connect the bot account, only system admins can connect the bot account.")
	}

	botTeamsUserID, err := p.store.MattermostToTeamsUserID(p.userID)
	if err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: unable to find the connected bot account")
		return &model.CommandResponse{}, nil
	}
	err = p.store.SetUserInfo(p.userID, botTeamsUserID, nil)
	if err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, fmt.Sprintf("Error: unable to disconnect the bot account, %s", err.Error()))
		return &model.CommandResponse{}, nil
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "The bot account has been disconnected")

	return &model.CommandResponse{}, nil
}

func getAutocompletePath(path string) string {
	return "plugins/" + pluginID + "/autocomplete/" + path
}
