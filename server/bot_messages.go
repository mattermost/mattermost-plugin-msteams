// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

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

// createAndStoreOAuthState creates an OAuth state for a bot message connect URL and stores it
func (p *Plugin) createAndStoreOAuthState(userID, channelID, postID string) string {
	stateSuffix := fmt.Sprintf("fromBotMessage:%s|%s", channelID, postID)
	stateID := model.NewId()
	state := fmt.Sprintf("%s_%s_%s", stateID, userID, stateSuffix)

	if err := p.store.StoreOAuth2State(state); err != nil {
		p.GetAPI().LogWarn("Error in storing the OAuth state", "error", err.Error())
	}

	return fmt.Sprintf(p.GetURL()+"/connect?post_id=%s&channel_id=%s&state_id=%s", postID, channelID, stateID)
}

func (p *Plugin) SendEphemeralConnectMessage(channelID string, userID string, message string) {
	postID := model.NewId()

	connectURL := p.createAndStoreOAuthState(userID, channelID, postID)
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

	p.API.LogInfo("Sent ephemeral connect message to user", "user_id", userID)
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

	connectURL := p.createAndStoreOAuthState(userID, channelID, post.Id)
	connectMessage := fmt.Sprintf("[Click here to connect your account](%s)", connectURL)
	if len(message) > 0 {
		connectMessage = message + " " + connectMessage
	}

	post.Message = connectMessage
	if err := p.apiClient.Post.UpdatePost(post); err != nil {
		p.GetAPI().LogWarn("Failed to update connection post", "user_id", userID, "error", err)
	}

	p.API.LogInfo("Sent connect message to user", "user_id", userID)
}

func (p *Plugin) SendWelcomeMessage(userID string) error {
	if err := p.botSendDirectPost(
		userID,
		p.makeWelcomeMessagePost(),
	); err != nil {
		return errors.Wrapf(err, "failed to send welcome message to user %s", userID)
	}

	p.API.LogInfo("Sent welcome message", "user_id", userID)

	return nil
}

func (p *Plugin) makeWelcomeMessagePost() *model.Post {
	return &model.Post{
		Message: "You'll now start receiving notifications in Mattermost for chats and group chats when you're away or offline in Microsoft Teams. To turn off these notifications in Mattermost, go to **Settings > MS Teams**. Learn more in our [documentation](https://mattermost.com/pl/ms-teams-plugin-end-user-learn-more).",
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
func (p *Plugin) notifyChat(recipientUserID string, actorDisplayName string, chatTopic string, chatSize int, chatLink string, message string, fileIds model.StringArray, skippedFileAttachments int) error {
	formattedMessage := formatNotificationMessage(actorDisplayName, chatTopic, chatSize, chatLink, message, len(fileIds), skippedFileAttachments)
	if formattedMessage == "" {
		return nil
	}

	if err := p.botSendDirectPost(recipientUserID, &model.Post{
		Message: formattedMessage,
		FileIds: fileIds,
	}); err != nil {
		p.GetAPI().LogWarn("Failed to send notification message", "user_id", recipientUserID, "error", err)
		return errors.Wrap(err, "error sending chat notification")
	}

	p.GetAPI().LogInfo("Sent chat notification message to user", "user_id", recipientUserID)
	return nil
}

func (p *Plugin) SendInviteMessage(user *model.User) error {
	message := fmt.Sprintf("@%s, you've been invited by your administrator to connect your Mattermost account with Microsoft Teams.", user.Username)
	invitePost := &model.Post{
		Message: message,
	}

	if err := p.botSendDirectPost(user.Id, invitePost); err != nil {
		p.GetAPI().LogWarn("Failed to send invitation message", "user_id", user.Id, "error", err)
		return errors.Wrapf(err, "error sending invitation bot message")
	}

	connectURL := p.createAndStoreOAuthState(user.Id, invitePost.ChannelId, invitePost.Id)

	invitePost.Message = fmt.Sprintf("%s [Click here to connect your account](%s).", invitePost.Message, connectURL)
	if err := p.apiClient.Post.UpdatePost(invitePost); err != nil {
		p.GetAPI().LogWarn("Failed to update invitation message", "user_id", user.Id, "error", err)
		return errors.Wrapf(err, "error updating invitation bot message")
	}

	p.GetAPI().LogInfo("Sent invitation message to user", "user_id", user.Id)

	return nil
}
