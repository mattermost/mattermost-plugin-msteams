// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Code generated by "make generate"
// DO NOT EDIT

package client_timerlayer

import (
	"io"
	"time"

	"github.com/microsoftgraph/msgraph-sdk-go/models"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
)

type ClientTimerLayer struct {
	msteams.Client
	metrics metrics.Metrics
}

func (c *ClientTimerLayer) Connect() error {
	start := time.Now()

	err := c.Client.Connect()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.Connect", success, elapsed)
	return err
}

func (c *ClientTimerLayer) CreateOrGetChatForUsers(usersIDs []string) (*clientmodels.Chat, error) {
	start := time.Now()

	result, err := c.Client.CreateOrGetChatForUsers(usersIDs)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.CreateOrGetChatForUsers", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) DeleteChatMessage(chatID string, msgID string) error {
	start := time.Now()

	err := c.Client.DeleteChatMessage(chatID, msgID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.DeleteChatMessage", success, elapsed)
	return err
}

func (c *ClientTimerLayer) DeleteMessage(teamID string, channelID string, parentID string, msgID string) error {
	start := time.Now()

	err := c.Client.DeleteMessage(teamID, channelID, parentID, msgID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.DeleteMessage", success, elapsed)
	return err
}

func (c *ClientTimerLayer) DeleteSubscription(subscriptionID string) error {
	start := time.Now()

	err := c.Client.DeleteSubscription(subscriptionID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.DeleteSubscription", success, elapsed)
	return err
}

func (c *ClientTimerLayer) GetChannelInTeam(teamID string, channelID string) (*clientmodels.Channel, error) {
	start := time.Now()

	result, err := c.Client.GetChannelInTeam(teamID, channelID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetChannelInTeam", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetChannelsInTeam(teamID string, filterQuery string) ([]*clientmodels.Channel, error) {
	start := time.Now()

	result, err := c.Client.GetChannelsInTeam(teamID, filterQuery)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetChannelsInTeam", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetChat(chatID string) (*clientmodels.Chat, error) {
	start := time.Now()

	result, err := c.Client.GetChat(chatID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetChat", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetChatMessage(chatID string, messageID string) (*clientmodels.Message, error) {
	start := time.Now()

	result, err := c.Client.GetChatMessage(chatID, messageID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetChatMessage", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetCodeSnippet(url string) (string, error) {
	start := time.Now()

	result, err := c.Client.GetCodeSnippet(url)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetCodeSnippet", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetFileContent(downloadURL string) ([]byte, error) {
	start := time.Now()

	result, err := c.Client.GetFileContent(downloadURL)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetFileContent", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetFileContentStream(downloadURL string, writer *io.PipeWriter, bufferSize int64) {
	start := time.Now()

	c.Client.GetFileContentStream(downloadURL, writer, bufferSize)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if true {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetFileContentStream", success, elapsed)

}

func (c *ClientTimerLayer) GetFileSizeAndDownloadURL(weburl string) (int64, string, error) {
	start := time.Now()

	result, resultVar1, err := c.Client.GetFileSizeAndDownloadURL(weburl)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetFileSizeAndDownloadURL", success, elapsed)
	return result, resultVar1, err
}

func (c *ClientTimerLayer) GetHostedFileContent(activityIDs *clientmodels.ActivityIds) ([]byte, error) {
	start := time.Now()

	result, err := c.Client.GetHostedFileContent(activityIDs)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetHostedFileContent", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetMe() (*clientmodels.User, error) {
	start := time.Now()

	result, err := c.Client.GetMe()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetMe", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetMessage(teamID string, channelID string, messageID string) (*clientmodels.Message, error) {
	start := time.Now()

	result, err := c.Client.GetMessage(teamID, channelID, messageID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetMessage", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetMyID() (string, error) {
	start := time.Now()

	result, err := c.Client.GetMyID()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetMyID", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetReply(teamID string, channelID string, messageID string, replyID string) (*clientmodels.Message, error) {
	start := time.Now()

	result, err := c.Client.GetReply(teamID, channelID, messageID, replyID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetReply", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetTeam(teamID string) (*clientmodels.Team, error) {
	start := time.Now()

	result, err := c.Client.GetTeam(teamID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetTeam", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetTeams(filterQuery string) ([]*clientmodels.Team, error) {
	start := time.Now()

	result, err := c.Client.GetTeams(filterQuery)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetTeams", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetUser(userID string) (*clientmodels.User, error) {
	start := time.Now()

	result, err := c.Client.GetUser(userID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetUser", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) GetUserAvatar(userID string) ([]byte, error) {
	start := time.Now()

	result, err := c.Client.GetUserAvatar(userID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.GetUserAvatar", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) ListChannels(teamID string) ([]clientmodels.Channel, error) {
	start := time.Now()

	result, err := c.Client.ListChannels(teamID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.ListChannels", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) ListSubscriptions() ([]*clientmodels.Subscription, error) {
	start := time.Now()

	result, err := c.Client.ListSubscriptions()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.ListSubscriptions", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) ListTeams() ([]clientmodels.Team, error) {
	start := time.Now()

	result, err := c.Client.ListTeams()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.ListTeams", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) ListUsers() ([]clientmodels.User, error) {
	start := time.Now()

	result, err := c.Client.ListUsers()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.ListUsers", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) RefreshSubscription(subscriptionID string) (*time.Time, error) {
	start := time.Now()

	result, err := c.Client.RefreshSubscription(subscriptionID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.RefreshSubscription", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) SendChat(chatID string, message string, parentMessage *clientmodels.Message, attachments []*clientmodels.Attachment, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	start := time.Now()

	result, err := c.Client.SendChat(chatID, message, parentMessage, attachments, mentions)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.SendChat", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) SendMessage(teamID string, channelID string, parentID string, message string) (*clientmodels.Message, error) {
	start := time.Now()

	result, err := c.Client.SendMessage(teamID, channelID, parentID, message)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.SendMessage", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) SendMessageWithAttachments(teamID string, channelID string, parentID string, message string, attachments []*clientmodels.Attachment, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	start := time.Now()

	result, err := c.Client.SendMessageWithAttachments(teamID, channelID, parentID, message, attachments, mentions)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.SendMessageWithAttachments", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) SetChatReaction(chatID string, messageID string, userID string, emoji string) (*clientmodels.Message, error) {
	start := time.Now()

	result, err := c.Client.SetChatReaction(chatID, messageID, userID, emoji)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.SetChatReaction", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) SetReaction(teamID string, channelID string, parentID string, messageID string, userID string, emoji string) (*clientmodels.Message, error) {
	start := time.Now()

	result, err := c.Client.SetReaction(teamID, channelID, parentID, messageID, userID, emoji)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.SetReaction", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) SubscribeToChannel(teamID string, channelID string, baseURL string, webhookSecret string, certificate string) (*clientmodels.Subscription, error) {
	start := time.Now()

	result, err := c.Client.SubscribeToChannel(teamID, channelID, baseURL, webhookSecret, certificate)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.SubscribeToChannel", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) SubscribeToChannels(baseURL string, webhookSecret string, pay bool, certificate string) (*clientmodels.Subscription, error) {
	start := time.Now()

	result, err := c.Client.SubscribeToChannels(baseURL, webhookSecret, pay, certificate)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.SubscribeToChannels", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) SubscribeToChats(baseURL string, webhookSecret string, pay bool, certificate string) (*clientmodels.Subscription, error) {
	start := time.Now()

	result, err := c.Client.SubscribeToChats(baseURL, webhookSecret, pay, certificate)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.SubscribeToChats", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) SubscribeToUserChats(user string, baseURL string, webhookSecret string, pay bool, certificate string) (*clientmodels.Subscription, error) {
	start := time.Now()

	result, err := c.Client.SubscribeToUserChats(user, baseURL, webhookSecret, pay, certificate)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.SubscribeToUserChats", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) UnsetChatReaction(chatID string, messageID string, userID string, emoji string) (*clientmodels.Message, error) {
	start := time.Now()

	result, err := c.Client.UnsetChatReaction(chatID, messageID, userID, emoji)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.UnsetChatReaction", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) UnsetReaction(teamID string, channelID string, parentID string, messageID string, userID string, emoji string) (*clientmodels.Message, error) {
	start := time.Now()

	result, err := c.Client.UnsetReaction(teamID, channelID, parentID, messageID, userID, emoji)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.UnsetReaction", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) UpdateChatMessage(chatID string, msgID string, message string, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	start := time.Now()

	result, err := c.Client.UpdateChatMessage(chatID, msgID, message, mentions)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.UpdateChatMessage", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) UpdateMessage(teamID string, channelID string, parentID string, msgID string, message string, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	start := time.Now()

	result, err := c.Client.UpdateMessage(teamID, channelID, parentID, msgID, message, mentions)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.UpdateMessage", success, elapsed)
	return result, err
}

func (c *ClientTimerLayer) UploadFile(teamID string, channelID string, filename string, filesize int, mimeType string, data io.Reader, chat *clientmodels.Chat) (*clientmodels.Attachment, error) {
	start := time.Now()

	result, err := c.Client.UploadFile(teamID, channelID, filename, filesize, mimeType, data, chat)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	c.metrics.ObserveMSGraphClientMethodDuration("Client.UploadFile", success, elapsed)
	return result, err
}

func New(childClient msteams.Client, metrics metrics.Metrics) *ClientTimerLayer {
	return &ClientTimerLayer{
		Client:  childClient,
		metrics: metrics,
	}
}
