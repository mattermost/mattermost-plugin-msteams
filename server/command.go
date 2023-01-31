package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
)

const msteamsCommand = "msteamssync"

func createMsteamsSyncCommand() *model.Command {
	return &model.Command{
		Trigger:          msteamsCommand,
		AutoComplete:     true,
		AutoCompleteDesc: "Start syncing a msteams channel with this mattermost channel",
		AutoCompleteHint: "[command]",
		Username:         botUsername,
		IconURL:          "https://upload.wikimedia.org/wikipedia/commons/c/c9/Microsoft_Office_Teams_%282018%E2%80%93present%29.svg",
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
		IconURL:      "https://upload.wikimedia.org/wikipedia/commons/c/c9/Microsoft_Office_Teams_%282018%E2%80%93present%29.svg",
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

	return cmdError(args.ChannelId, fmt.Sprintf("getUser() threw error: %s", errors.New("you need at least one command")))
}

func (p *Plugin) executeLinkCommand(c *plugin.Context, args *model.CommandArgs, parameters []string) (*model.CommandResponse, *model.AppError) {
	if len(parameters) < 2 {
		return cmdError(args.ChannelId, fmt.Sprintf("getUser() threw error: %s", errors.New("you need at least two parameters")))
	}

	// TODO: Check User Permissions
	// user, appErr := p.API.GetUser(args.UserId)
	// if appErr != nil {
	// 	return cmdError(args.ChannelId, fmt.Sprintf("getUser() threw error: %s", appErr))
	// }

	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}

	channelsLinkedData, appErr := p.API.KVGet("channelsLinked")
	if appErr != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}
	channelsLinked := map[string]ChannelLink{}
	json.Unmarshal(channelsLinkedData, &channelsLinked)
	_, ok := channelsLinked[channel.TeamId+":"+channel.Id]
	if ok {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", errors.New("already exiting link, please unlink and link again")))
	}

	ct := p.msteamsAppClient.Teams().ID(parameters[0]).Channels().ID(parameters[1]).Request()
	_, err := ct.Get(p.msteamsAppClientCtx)
	if err != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("msteams channel not found: %s", err))
	}

	channelsLinked[channel.TeamId+":"+channel.Id] = ChannelLink{
		MattermostTeam:    channel.TeamId,
		MattermostChannel: channel.Id,
		MSTeamsTeam:       parameters[0],
		MSTeamsChannel:    parameters[1],
	}

	channelsLinkedData, err = json.Marshal(channelsLinked)
	if err != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", err))
	}

	appErr = p.API.KVSet("channelsLinked", channelsLinkedData)
	if appErr != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}

	p.restart()

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         "The msteams channel is now linked to this mattermost channel",
		Username:     botDisplayName,
		IconURL:      "https://upload.wikimedia.org/wikipedia/commons/c/c9/Microsoft_Office_Teams_%282018%E2%80%93present%29.svg",
	}, nil
}

func (p *Plugin) executeUnlinkCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	// TODO: Check User Permissions
	// user, appErr := p.API.GetUser(args.UserId)
	// if appErr != nil {
	// 	return cmdError(args.ChannelId, fmt.Sprintf("getUser() threw error: %s", appErr))
	// }

	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}

	channelsLinkedData, appErr := p.API.KVGet("channelsLinked")
	if appErr != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}
	var channelsLinked map[string]ChannelLink
	json.Unmarshal(channelsLinkedData, &channelsLinked)
	_, ok := channelsLinked[channel.TeamId+":"+channel.Id]
	if !ok {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", errors.New("the link doesnt exists")))
	}

	delete(channelsLinked, channel.TeamId+":"+channel.Id)
	channelsLinkedData, err := json.Marshal(channelsLinked)
	if err != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", err))
	}

	appErr = p.API.KVSet("channelsLinked", channelsLinkedData)
	if appErr != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}

	p.restart()

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         "The msteams channel is no longer linked to this mattermost channel",
		Username:     botDisplayName,
		IconURL:      "https://upload.wikimedia.org/wikipedia/commons/c/c9/Microsoft_Office_Teams_%282018%E2%80%93present%29.svg",
	}, nil
}

func (p *Plugin) executeShowCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	// TODO: Check User Permissions
	// user, appErr := p.API.GetUser(args.UserId)
	// if appErr != nil {
	// 	return cmdError(args.ChannelId, fmt.Sprintf("getUser() threw error: %s", appErr))
	// }

	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}

	channelsLinkedData, appErr := p.API.KVGet("channelsLinked")
	if appErr != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}
	var channelsLinked map[string]ChannelLink
	json.Unmarshal(channelsLinkedData, &channelsLinked)
	link, ok := channelsLinked[channel.TeamId+":"+channel.Id]
	if !ok {
		return cmdError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", errors.New("the link doesnt exists")))
	}

	ctTeam := p.msteamsAppClient.Teams().ID(link.MSTeamsTeam).Request()
	msteamsTeam, err := ctTeam.Get(p.msteamsAppClientCtx)
	if err != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("msteams channel not found: %s", err))
	}

	ctChannel := p.msteamsAppClient.Teams().ID(link.MSTeamsTeam).Channels().ID(link.MSTeamsChannel).Request()
	msteamsChannel, err := ctChannel.Get(p.msteamsAppClientCtx)
	if err != nil {
		return cmdError(args.ChannelId, fmt.Sprintf("msteams channel not found: %s", err))
	}

	text := fmt.Sprintf(
		"This channel is linked to the MS Teams Channel \"%s\" (with id: %s) in the Team \"%s\" (with the id: %s).",
		*msteamsTeam.DisplayName,
		*msteamsTeam.ID,
		*msteamsChannel.DisplayName,
		*msteamsChannel.ID,
	)
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         text,
		Username:     botDisplayName,
		IconURL:      "https://upload.wikimedia.org/wikipedia/commons/c/c9/Microsoft_Office_Teams_%282018%E2%80%93present%29.svg",
	}, nil
}
