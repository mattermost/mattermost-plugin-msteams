package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

const msteamsCommand = "msteamssync"
const msteamsLogoURL = "https://upload.wikimedia.org/wikipedia/commons/c/c9/Microsoft_Office_Teams_%282018%E2%80%93present%29.svg"

func createMsteamsSyncCommand() *model.Command {
	return &model.Command{
		Trigger:          msteamsCommand,
		AutoComplete:     true,
		AutoCompleteDesc: "Start syncing a msteams channel with this mattermost channel",
		AutoCompleteHint: "[command]",
		Username:         botUsername,
		IconURL:          msteamsLogoURL,
		DisplayName:      botDisplayName,
		AutocompleteData: getAutocompleteData(),
	}
}

func cmdError(channelID string, detailedError string) (*model.CommandResponse, *model.AppError) {
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		ChannelId:    channelID,
		Text:         detailedError,
		Username:     botDisplayName,
		IconURL:      msteamsLogoURL,
	}, nil
}

func getAutocompleteData() *model.AutocompleteData {
	cmd := model.NewAutocompleteData(msteamsCommand, "[command]", "Manage MS Teams linked channels")

	link := model.NewAutocompleteData("link", "[msteams-team-id] [msteams-channel-id]", "Link current channel to a MS Teams channel")
	link.AddTextArgument("MS Teams Team ID", "[msteams-team-id]", "")
	link.AddTextArgument("MS Teams Channel ID", "[msteams-channel-id]", "")
	cmd.AddCommand(link)

	unlink := model.NewAutocompleteData("unlink", "", "Unlink the current channel from the MS Teams channel")
	cmd.AddCommand(unlink)

	show := model.NewAutocompleteData("show", "", "Show MS Teams linked channel")
	cmd.AddCommand(show)

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

	return cmdError(args.ChannelId, "Unknown command. Valid options: link, unlink and show.")
}

func (p *Plugin) executeLinkCommand(c *plugin.Context, args *model.CommandArgs, parameters []string) (*model.CommandResponse, *model.AppError) {
	if len(parameters) < 2 {
		return cmdError(args.ChannelId, "Invalid link command, please pass the MS Teams team id and channel id as parameters.")
	}

	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return cmdError(args.ChannelId, "Unable to get the current channel information.")
	}

	canLinkChannel := channel.Type == model.ChannelTypeOpen && p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManagePublicChannelProperties)
	canLinkChannel = canLinkChannel || (channel.Type == model.ChannelTypePrivate && p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManagePrivateChannelProperties))
	if !canLinkChannel {
		return cmdError(args.ChannelId, "Unable to link the channel, you has to be a channel admin to link it.")
	}

	channelsLinkedData, appErr := p.API.KVGet("channelsLinked")
	if appErr != nil {
		return cmdError(args.ChannelId, "Unable to get the linked channels information, please try again.")
	}
	channelsLinked := map[string]ChannelLink{}
	json.Unmarshal(channelsLinkedData, &channelsLinked)
	_, ok := channelsLinked[channel.TeamId+":"+channel.Id]
	if ok {
		return cmdError(args.ChannelId, "A link for this channel already exists, please remove unlink the channel before you link a new one")
	}

	_, err := p.msteamsAppClient.GetChannel(parameters[0], parameters[1])
	if err != nil {
		return cmdError(args.ChannelId, "MS Teams channel not found.")
	}

	channelsLinked[channel.TeamId+":"+channel.Id] = ChannelLink{
		MattermostTeam:    channel.TeamId,
		MattermostChannel: channel.Id,
		MSTeamsTeam:       parameters[0],
		MSTeamsChannel:    parameters[1],
	}

	channelsLinkedData, err = json.Marshal(channelsLinked)
	if err != nil {
		return cmdError(args.ChannelId, "Unable to store the new link, please try again.")
	}

	appErr = p.API.KVSet("channelsLinked", channelsLinkedData)
	if appErr != nil {
		return cmdError(args.ChannelId, "Unable to store the new link, please try again.")
	}

	p.restart()

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         "The msteams channel is now linked to this mattermost channel",
		Username:     botDisplayName,
		IconURL:      msteamsLogoURL,
	}, nil
}

func (p *Plugin) executeUnlinkCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return cmdError(args.ChannelId, "Unable to get the current channel information.")
	}

	canLinkChannel := channel.Type == model.ChannelTypeOpen && p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManagePublicChannelProperties)
	canLinkChannel = canLinkChannel || (channel.Type == model.ChannelTypePrivate && p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManagePrivateChannelProperties))
	if !canLinkChannel {
		return cmdError(args.ChannelId, "Unable to unlink the channel, you has to be a channel admin to unlink it.")
	}

	channelsLinkedData, appErr := p.API.KVGet("channelsLinked")
	if appErr != nil {
		return cmdError(args.ChannelId, "Unable to get the linked channels information, please try again.")
	}
	var channelsLinked map[string]ChannelLink
	json.Unmarshal(channelsLinkedData, &channelsLinked)
	_, ok := channelsLinked[channel.TeamId+":"+channel.Id]
	if !ok {
		return cmdError(args.ChannelId, "Link doesn't exists.")
	}

	delete(channelsLinked, channel.TeamId+":"+channel.Id)
	channelsLinkedData, err := json.Marshal(channelsLinked)
	if err != nil {
		return cmdError(args.ChannelId, "Unable to store the new link, please try again.")
	}

	appErr = p.API.KVSet("channelsLinked", channelsLinkedData)
	if appErr != nil {
		return cmdError(args.ChannelId, "Unable to store the new link, please try again.")
	}

	p.restart()

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         "The msteams channel is no longer linked to this mattermost channel",
		Username:     botDisplayName,
		IconURL:      msteamsLogoURL,
	}, nil
}

func (p *Plugin) executeShowCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return cmdError(args.ChannelId, "Unable to get the current channel information.")
	}

	channelsLinkedData, appErr := p.API.KVGet("channelsLinked")
	if appErr != nil {
		return cmdError(args.ChannelId, "Unable to get the linked channels information, please try again.")
	}
	var channelsLinked map[string]ChannelLink
	json.Unmarshal(channelsLinkedData, &channelsLinked)
	link, ok := channelsLinked[channel.TeamId+":"+channel.Id]
	if !ok {
		return cmdError(args.ChannelId, "Link doesn't exists.")
	}

	msteamsTeam, err := p.msteamsAppClient.GetTeam(link.MSTeamsTeam)
	if err != nil {
		return cmdError(args.ChannelId, "Unable to get the MS Teams team information.")
	}

	msteamsChannel, err := p.msteamsAppClient.GetChannel(link.MSTeamsTeam, link.MSTeamsChannel)
	if err != nil {
		return cmdError(args.ChannelId, "Unable to get the MS Teams channel information.")
	}

	text := fmt.Sprintf(
		"This channel is linked to the MS Teams Channel \"%s\" (with id: %s) in the Team \"%s\" (with the id: %s).",
		msteamsTeam.DisplayName,
		msteamsTeam.ID,
		msteamsChannel.DisplayName,
		msteamsChannel.ID,
	)
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         text,
		Username:     botDisplayName,
		IconURL:      msteamsLogoURL,
	}, nil
}
