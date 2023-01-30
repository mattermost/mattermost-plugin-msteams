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
		AutoCompleteHint: "[msteams-teamId] [msteams-channelId]",
	}
}

func linkError(channelID string, detailedError string) (*model.CommandResponse, *model.AppError) {
	return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			ChannelId:    channelID,
			Text:         "We could not link the channel with msteams channel.",
		}, &model.AppError{
			Message:       "We could not link the channel with msteams channel.",
			DetailedError: detailedError,
		}
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
		return p.executeUnlinkCommand(c, args, parameters)
	}

	return linkError(args.ChannelId, fmt.Sprintf("getUser() threw error: %s", errors.New("you need at least one command")))
}

type ChannelLink struct {
	MattermostTeam    string
	MattermostChannel string
	MSTeamsTeam       string
	MSTeamsChannel    string
}

func (p *Plugin) executeLinkCommand(c *plugin.Context, args *model.CommandArgs, parameters []string) (*model.CommandResponse, *model.AppError) {
	if len(parameters) < 2 {
		return linkError(args.ChannelId, fmt.Sprintf("getUser() threw error: %s", errors.New("you need at least two parameters")))
	}

	// TODO: Check User Permissions
	// user, appErr := p.API.GetUser(args.UserId)
	// if appErr != nil {
	// 	return linkError(args.ChannelId, fmt.Sprintf("getUser() threw error: %s", appErr))
	// }

	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return linkError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}

	channelsLinkedData, appErr := p.API.KVGet("channelsLinked")
	if appErr != nil {
		return linkError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}
	var channelsLinked map[string]ChannelLink
	json.Unmarshal(channelsLinkedData, &channelsLinked)
	_, ok := channelsLinked[channel.TeamId+":"+channel.Id]
	if ok {
		return linkError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", errors.New("already exiting link, please unlink and link again")))
	}

	channelsLinked[channel.TeamId+":"+channel.Id] = ChannelLink{
		MattermostTeam:    channel.TeamId,
		MattermostChannel: channel.Id,
		MSTeamsTeam:       parameters[0],
		MSTeamsChannel:    parameters[1],
	}

	channelsLinkedData, err := json.Marshal(channelsLinked)
	if err != nil {
		return linkError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", err))
	}

	appErr = p.API.KVSet("channelsLinked", channelsLinkedData)
	if appErr != nil {
		return linkError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}

	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeUnlinkCommand(c *plugin.Context, args *model.CommandArgs, parameters []string) (*model.CommandResponse, *model.AppError) {
	// TODO: Check User Permissions
	// user, appErr := p.API.GetUser(args.UserId)
	// if appErr != nil {
	// 	return linkError(args.ChannelId, fmt.Sprintf("getUser() threw error: %s", appErr))
	// }

	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return linkError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}

	channelsLinkedData, appErr := p.API.KVGet("channelsLinked")
	if appErr != nil {
		return linkError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}
	var channelsLinked map[string]ChannelLink
	json.Unmarshal(channelsLinkedData, &channelsLinked)
	_, ok := channelsLinked[channel.TeamId+":"+channel.Id]
	if !ok {
		return linkError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", errors.New("the link doesnt exists")))
	}

	delete(channelsLinked, channel.TeamId+":"+channel.Id)
	channelsLinkedData, err := json.Marshal(channelsLinked)
	if err != nil {
		return linkError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", err))
	}

	appErr = p.API.KVSet("channelsLinked", channelsLinkedData)
	if appErr != nil {
		return linkError(args.ChannelId, fmt.Sprintf("getChannel() threw error: %s", appErr))
	}

	return &model.CommandResponse{}, nil
}
