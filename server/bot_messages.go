package main

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

func (p *Plugin) botSendDirectMessage(userID, message string) error {
	channel, err := p.apiClient.Channel.GetDirect(userID, p.userID)
	if err != nil {
		return errors.Wrapf(err, "failed to get bot DM channel with user_id %s", userID)
	}

	return p.apiClient.Post.CreatePost(&model.Post{
		Message:   message,
		UserId:    p.userID,
		ChannelId: channel.Id,
	})
}

func (p *Plugin) handlePromptForConnection(userID, channelID string) {
	message := fmt.Sprintf("[Click here to connect your account](%s).", p.GetURL()+"/connect")
	p.sendBotEphemeralPost(userID, channelID, "Some users in this conversation rely on Microsoft Teams to receive your messages, but your account isn't connected. "+message)
}

func (p *Plugin) notifyUserDisconnected(userID string) {
	channel, appErr := p.API.GetDirectChannel(userID, p.GetBotUserID())
	if appErr != nil {
		p.API.LogWarn("Unable to get direct channel for send message to user", "user_id", userID, "error", appErr.Error())
		return
	}

	connectURL := p.GetURL() + "/connect"
	_, appErr = p.API.CreatePost(&model.Post{
		UserId:    p.GetBotUserID(),
		ChannelId: channel.Id,
		Message:   "Your connection to Microsoft Teams has been lost. " + fmt.Sprintf("[Click here to reconnect your account](%s).", connectURL),
	})
	if appErr != nil {
		p.API.LogWarn("Unable to send direct message to user", "user_id", userID, "error", appErr.Error())
	}
}

func (p *Plugin) NotifyFileAttachmentError(userID, channelID string) {
	_ = p.GetAPI().SendEphemeralPost(userID, &model.Post{
		ChannelId: channelID,
		UserId:    p.GetBotUserID(),
		Message:   "Some images could not be delivered because they exceeded the maximum resolution and/or size allowed.",
	})
}

func (p *Plugin) notifyAttachmentsNotSupportedFromMattermost(post *model.Post) {
	_, appErr := p.GetAPI().CreatePost(&model.Post{
		ChannelId: post.ChannelId,
		UserId:    p.GetBotUserID(),
		Message:   "Attachments sent from Mattermost aren't yet delivered to Microsoft Teams.",
		CreateAt:  post.CreateAt,
	})
	if appErr != nil {
		p.GetAPI().LogWarn("Failed to notify channel of skipped attachment", "channel_id", post.ChannelId, "post_id", post.Id, "error", appErr)
	}
}

func (p *Plugin) NotifyAttachmentsNotSupportedFromTeams(post *model.Post) {
	_, appErr := p.GetAPI().CreatePost(&model.Post{
		ChannelId: post.ChannelId,
		UserId:    p.GetBotUserID(),
		Message:   "Attachments sent from Microsoft Teams aren't delivered to Mattermost.",
		// Anchor the post immediately after (never before) the post that was created.
		CreateAt: post.CreateAt + 1,
	})
	if appErr != nil {
		p.GetAPI().LogWarn("Failed to notify channel of skipped attachment", "channel_id", post.ChannelId, "post_id", post.Id, "error", appErr)
	}
}

func (p *Plugin) NotifyUpdatedAttachmentsNotSupportedFromTeams(post *model.Post) {
	_, appErr := p.GetAPI().CreatePost(&model.Post{
		ChannelId: post.ChannelId,
		UserId:    p.GetBotUserID(),
		Message:   "Attachments added to an existing post in Microsoft Teams aren't delivered to Mattermost.",
		// Anchor the post immediately after (never before) the post that was edited.
		CreateAt: post.CreateAt + 1,
	})
	if appErr != nil {
		p.GetAPI().LogWarn("Failed to notify channel of skipped attachment", "channel_id", post.ChannelId, "post_id", post.Id, "error", appErr)
	}
}
