package main

import (
	"fmt"

	"github.com/mattermost/mattermost-server/v6/model"
)

// DM posts a simple Direct Message to the specified user
func (p *Plugin) DM(mattermostUserID, format string, args ...interface{}) (string, error) {
	botID := p.GetBotUserID()
	channel, err := p.API.GetDirectChannel(mattermostUserID, botID)
	if err != nil {
		p.API.LogError("Couldn't get bot's DM channel", "userID", mattermostUserID, "error", err.Error())
		return "", err
	}

	post := &model.Post{
		ChannelId: channel.Id,
		UserId:    botID,
		Message:   fmt.Sprintf(format, args...),
	}

	sentPost, err := p.API.CreatePost(post)
	if err != nil {
		p.API.LogError("Error occurred while creating post", "error", err.Error())
		return "", err
	}

	return sentPost.Id, nil
}
