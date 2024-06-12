package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

func (p *Plugin) botSendDirectMessage(userID, message string) error {
	return p.botSendDirectPost(userID, &model.Post{
		Message: message,
	})
}

func (p *Plugin) botSendDirectPost(userID string, post *model.Post) error {
	channel, err := p.apiClient.Channel.GetDirect(userID, p.botUserID)
	if err != nil {
		return errors.Wrapf(err, "failed to get bot DM channel with user_id %s", userID)
	}

	post.ChannelId = channel.Id
	post.UserId = p.botUserID

	// Force posts from the bot to render the user profile icon each time instead of collapsing
	// adjacent posts. This helps draw attention to each individual post.
	post.AddProp("from_webhook", "true")
	post.AddProp("use_user_icon", "true")

	return p.apiClient.Post.CreatePost(post)
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

func (p *Plugin) SendWelcomeMessageWithNotificationAction(userID string) error {
	if err := p.botSendDirectPost(
		userID,
		p.makeWelcomeMessageWithNotificationActionPost(),
	); err != nil {
		return errors.Wrapf(err, "failed to send welcome message to user %s", userID)
	}

	return nil
}

func (p *Plugin) makeWelcomeMessageWithNotificationActionPost() *model.Post {
	msg := []string{
		"**Notifications from chats and group chats**",
		"With this feature enabled, you will get notified by the MS Teams bot whenever you receive a message from a chat or group chat in Teams.",
		fmt.Sprintf("![enable notifications picture](%s/static/enable_notifications.gif)", p.GetRelativeURL()),
	}

	return &model.Post{
		Message: strings.Join(msg, "\n\n"),
		Props: model.StringInterface{
			"attachments": []*model.SlackAttachment{
				{
					Actions: []*model.PostAction{
						{
							Integration: &model.PostActionIntegration{
								URL: fmt.Sprintf("%s/enable-notifications", p.GetRelativeURL()),
							},
							Name:  "Enable Notifications",
							Style: "primary",
							Type:  model.PostActionTypeButton,
						},
						{
							Integration: &model.PostActionIntegration{
								URL: fmt.Sprintf("%s/dismiss-notifications", p.GetRelativeURL()),
							},
							Name:  "Dismiss",
							Style: "default",
							Type:  model.PostActionTypeButton,
						},
					},
				},
			},
		},
	}
}

// formatNotificationMessage formats the message about a notification of a chat received on Teams.
func formatNotificationMessage(actorDisplayName string, chatTopic string, chatSize int, chatLink string, message string, attachmentCount int) string {
	message = strings.TrimSpace(message)
	if message == "" && attachmentCount == 0 {
		return ""
	}

	var preamble string

	var chatTopicDesc string
	if chatTopic != "" {
		chatTopicDesc = ": " + chatTopic
	}

	if chatSize <= 1 {
		return ""
	} else if chatSize == 2 {
		preamble = fmt.Sprintf("**%s** messaged you in an [MS Teams chat%s](%s):", actorDisplayName, chatTopicDesc, chatLink)
	} else if chatSize == 3 {
		preamble = fmt.Sprintf("**%s** messaged you and 1 other user in an [MS Teams group chat%s](%s):", actorDisplayName, chatTopicDesc, chatLink)
	} else {
		preamble = fmt.Sprintf("**%s** messaged you and %d other users in an [MS Teams group chat%s](%s):", actorDisplayName, chatSize-2, chatTopicDesc, chatLink)
	}
	preamble += "\n"

	if message != "" {
		message = "> " + strings.ReplaceAll(message, "\n", "\n> ")
	}

	attachmentsNotice := ""
	if attachmentCount > 0 {
		if len(message) > 0 {
			attachmentsNotice += "\n"
		}
		attachmentsNotice += "\n*"
		if attachmentCount == 1 {
			attachmentsNotice += "This message was originally sent with one attachment."
		} else {
			attachmentsNotice += fmt.Sprintf("This message was originally sent with %d attachments.", attachmentCount)
		}
		attachmentsNotice += "*"
	}

	formattedMessage := fmt.Sprintf(`%s%s%s`, preamble, message, attachmentsNotice)

	return formattedMessage
}

// notifyMessage sends the given receipient a notification of a chat received on Teams.
func (p *Plugin) notifyChat(recipientUserID string, actorDisplayName string, chatTopic string, chatSize int, chatLink string, message string, attachmentCount int) {
	formattedMessage := formatNotificationMessage(actorDisplayName, chatTopic, chatSize, chatLink, message, attachmentCount)

	if formattedMessage == "" {
		return
	}

	if err := p.botSendDirectMessage(recipientUserID, formattedMessage); err != nil {
		p.GetAPI().LogWarn("Failed to send notification message", "user_id", recipientUserID, "error", err)
	}
}
