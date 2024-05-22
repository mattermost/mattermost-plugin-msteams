package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

func (p *Plugin) botSendDirectMessage(userID, message string) error {
	channel, err := p.apiClient.Channel.GetDirect(userID, p.botUserID)
	if err != nil {
		return errors.Wrapf(err, "failed to get bot DM channel with user_id %s", userID)
	}

	return p.apiClient.Post.CreatePost(&model.Post{
		Message:   message,
		UserId:    p.botUserID,
		ChannelId: channel.Id,
	})
}

func (p *Plugin) handlePromptForConnection(userID, channelID string) {
	// For now, don't display the connect message

	// message := "Some users in this conversation rely on Microsoft Teams to receive your messages, but your account isn't connected. "
	// p.SendEphemeralConnectMessage(channelID, userID, message)
}

func (p *Plugin) SendEphemeralConnectMessage(channelID string, userID string, message string) {
	postID := model.NewId()
	connectURL := fmt.Sprintf(p.GetURL()+"/connect?post_id=%s&channel_id=%s", postID, channelID)
	connectMessage := fmt.Sprintf("[Click here to connect your account](%s)", connectURL)
	if len(message) > 0 {
		connectMessage = message + " " + connectMessage
	}
	post := &model.Post{
		Id:        postID,
		ChannelId: channelID,
		UserId:    p.GetBotUserID(),
		Message:   connectMessage,
	}
	p.API.SendEphemeralPost(userID, post)
}
func (p *Plugin) SendConnectMessage(channelID string, userID string, message string) {
	post := &model.Post{
		Message:   message,
		UserId:    p.GetBotUserID(),
		ChannelId: channelID,
	}
	if err := p.apiClient.Post.CreatePost(post); err != nil {
		p.GetAPI().LogWarn("Failed to create connection post", "user_id", userID, "error", err)
		return
	}

	connectURL := fmt.Sprintf(p.GetURL()+"/connect?post_id=%s&channel_id=%s", post.Id, channelID)
	connectMessage := fmt.Sprintf("[Click here to connect your account](%s)", connectURL)
	if len(message) > 0 {
		connectMessage = message + " " + connectMessage
	}

	post.Message = connectMessage
	if err := p.apiClient.Post.UpdatePost(post); err != nil {
		p.GetAPI().LogWarn("Failed to update connection post", "user_id", userID, "error", err)
	}
}

func (p *Plugin) SendConnectBotMessage(channelID string, userID string) {
	postID := model.NewId()
	connectURL := fmt.Sprintf(p.GetURL()+"/connect?isBot&post_id=%s&channel_id=%s", postID, channelID)
	connectMessage := fmt.Sprintf("[Click here to connect the bot account](%s)", connectURL)
	post := &model.Post{
		Id:        postID,
		ChannelId: channelID,
		UserId:    p.GetBotUserID(),
		Message:   connectMessage,
	}
	p.API.SendEphemeralPost(userID, post)
}

const userChoseMattermostPrimaryMessage = "You’ve chosen Mattermost as your primary platform: you’ll receive Microsoft Teams messages and notifications in Mattermost. Consider [disabling MS Teams notifications](https://support.microsoft.com/en-us/office/manage-notifications-in-microsoft-teams-1cc31834-5fe5-412b-8edb-43fecc78413d) to avoid duplicated notifications."

func (p *Plugin) notifyUserMattermostPrimary(userID string) {
	if err := p.botSendDirectMessage(userID, userChoseMattermostPrimaryMessage); err != nil {
		p.GetAPI().LogWarn("Failed to notify user is Mattermost primary", "user_id", userID, "error", err)
	}
}

const userChoseTeamsPrimaryMessage = "You’ve chosen Microsoft Teams as your primary platform: your Mattermost notifications for DMs and GMs are muted, and you’ll receive chats from Mattermost in Microsoft Teams."

func (p *Plugin) notifyUserTeamsPrimary(userID string) {
	if err := p.botSendDirectMessage(userID, userChoseTeamsPrimaryMessage); err != nil {
		p.GetAPI().LogWarn("Failed to notify user is Teams primary", "user_id", userID, "error", err)
	}
}

// notifyMessage sends the given receipient a notification of a chat received on Teams.
func (p *Plugin) notifyChat(recipientUserID string, actorDisplayName string, chatSize int, chatLink string, message string) {
	var preamble string

	if chatSize <= 1 {
		return
	} else if chatSize == 2 {
		preamble = fmt.Sprintf("%s messaged you in MS Teams:", actorDisplayName)
	} else if chatSize == 3 {
		preamble = fmt.Sprintf("%s messaged you and 1 other user in MS Teams:", actorDisplayName)
	} else {
		preamble = fmt.Sprintf("%s messaged you and %d other users in MS Teams:", actorDisplayName, chatSize-2)
	}

	message = "> " + strings.ReplaceAll(message, "\n", "\n>")

	formattedMessage := fmt.Sprintf(`%s
%s

**[Respond in Microsoft Teams ↗️](%s)**`, preamble, message, chatLink)

	if err := p.botSendDirectMessage(recipientUserID, formattedMessage); err != nil {
		p.GetAPI().LogWarn("Failed to send notification message", "user_id", recipientUserID, "error", err)
	}
}
