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
							Name:  "Enable notifications",
							Style: "primary",
							Type:  model.PostActionTypeButton,
						},
						{
							Integration: &model.PostActionIntegration{
								URL: fmt.Sprintf("%s/disable-notifications", p.GetRelativeURL()),
							},
							Name:  "Disable",
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
func formatNotificationMessage(actorDisplayName string, chatTopic string, chatSize int, chatLink string, message string, attachmentCount int, skippedFileAttachments int) string {
	message = strings.TrimSpace(message)
	if message == "" && attachmentCount == 0 && skippedFileAttachments == 0 {
		return ""
	}

	var messageComponents []string

	var chatTopicDesc string
	if chatTopic != "" {
		chatTopicDesc = ": " + chatTopic
	}

	// Handle the preamble
	if chatSize <= 1 {
		return ""
	} else if chatSize == 2 {
		messageComponents = append(messageComponents,
			fmt.Sprintf("**%s** messaged you in an [MS Teams chat%s](%s):", actorDisplayName, chatTopicDesc, chatLink),
		)
	} else if chatSize == 3 {
		messageComponents = append(messageComponents,
			fmt.Sprintf("**%s** messaged you and 1 other user in an [MS Teams group chat%s](%s):", actorDisplayName, chatTopicDesc, chatLink),
		)
	} else {
		messageComponents = append(messageComponents,
			fmt.Sprintf("**%s** messaged you and %d other users in an [MS Teams group chat%s](%s):", actorDisplayName, chatSize-2, chatTopicDesc, chatLink),
		)
	}

	// Handle the message itself
	if len(message) > 0 {
		messageComponents = append(messageComponents,
			fmt.Sprintf("> %s", strings.ReplaceAll(message, "\n", "\n> ")),
		)
	}

	if skippedFileAttachments > 0 {
		messageComponents = append(messageComponents,
			"\n*Some file attachments from this message could not be delivered.*",
		)
	}

	formattedMessage := strings.Join(messageComponents, "\n")

	return formattedMessage
}

// notifyMessage sends the given receipient a notification of a chat received on Teams.
func (p *Plugin) notifyChat(recipientUserID string, actorDisplayName string, chatTopic string, chatSize int, chatLink string, message string, fileIds model.StringArray, skippedFileAttachments int) {
	formattedMessage := formatNotificationMessage(actorDisplayName, chatTopic, chatSize, chatLink, message, len(fileIds), skippedFileAttachments)
	if formattedMessage == "" {
		return
	}

	if err := p.botSendDirectPost(recipientUserID, &model.Post{
		Message: formattedMessage,
		FileIds: fileIds,
	}); err != nil {
		p.GetAPI().LogWarn("Failed to send notification message", "user_id", recipientUserID, "error", err)
	}
}
