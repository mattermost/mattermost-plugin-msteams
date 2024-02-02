// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Code generated by "make generate"
// DO NOT EDIT

package client_disconnectionlayer

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
)

type ClientDisconnectionLayer struct {
	msteams.Client
	userID       string
	onDisconnect func(userID string)
}

func (c *ClientDisconnectionLayer) Connect() error {
	err := c.Client.Connect()
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return err
}

func (c *ClientDisconnectionLayer) CreateOrGetChatForUsers(usersIDs []string) (*clientmodels.Chat, error) {
	result, err := c.Client.CreateOrGetChatForUsers(usersIDs)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) DeleteChatMessage(chatID string, msgID string) error {
	err := c.Client.DeleteChatMessage(chatID, msgID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return err
}

func (c *ClientDisconnectionLayer) DeleteMessage(teamID string, channelID string, parentID string, msgID string) error {
	err := c.Client.DeleteMessage(teamID, channelID, parentID, msgID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return err
}

func (c *ClientDisconnectionLayer) DeleteSubscription(subscriptionID string) error {
	err := c.Client.DeleteSubscription(subscriptionID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return err
}

func (c *ClientDisconnectionLayer) GetChannelInTeam(teamID string, channelID string) (*clientmodels.Channel, error) {
	result, err := c.Client.GetChannelInTeam(teamID, channelID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetChannelsInTeam(teamID string, filterQuery string) ([]*clientmodels.Channel, error) {
	result, err := c.Client.GetChannelsInTeam(teamID, filterQuery)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetChat(chatID string) (*clientmodels.Chat, error) {
	result, err := c.Client.GetChat(chatID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetChatMessage(chatID string, messageID string) (*clientmodels.Message, error) {
	result, err := c.Client.GetChatMessage(chatID, messageID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetCodeSnippet(url string) (string, error) {
	result, err := c.Client.GetCodeSnippet(url)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetFileContent(downloadURL string) ([]byte, error) {
	result, err := c.Client.GetFileContent(downloadURL)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetFileSizeAndDownloadURL(weburl string) (int64, string, error) {
	result, resultVar1, err := c.Client.GetFileSizeAndDownloadURL(weburl)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, resultVar1, err
}

func (c *ClientDisconnectionLayer) GetHostedFileContent(activityIDs *clientmodels.ActivityIds) ([]byte, error) {
	result, err := c.Client.GetHostedFileContent(activityIDs)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetMe() (*clientmodels.User, error) {
	result, err := c.Client.GetMe()
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetMessage(teamID string, channelID string, messageID string) (*clientmodels.Message, error) {
	result, err := c.Client.GetMessage(teamID, channelID, messageID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetMyID() (string, error) {
	result, err := c.Client.GetMyID()
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetReply(teamID string, channelID string, messageID string, replyID string) (*clientmodels.Message, error) {
	result, err := c.Client.GetReply(teamID, channelID, messageID, replyID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetTeam(teamID string) (*clientmodels.Team, error) {
	result, err := c.Client.GetTeam(teamID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetTeams(filterQuery string) ([]*clientmodels.Team, error) {
	result, err := c.Client.GetTeams(filterQuery)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetUser(userID string) (*clientmodels.User, error) {
	result, err := c.Client.GetUser(userID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) GetUserAvatar(userID string) ([]byte, error) {
	result, err := c.Client.GetUserAvatar(userID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) ListChannelMessages(teamID string, channelID string, since time.Time) ([]*clientmodels.Message, error) {
	result, err := c.Client.ListChannelMessages(teamID, channelID, since)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) ListChannels(teamID string) ([]clientmodels.Channel, error) {
	result, err := c.Client.ListChannels(teamID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) ListChatMessages(chatID string, since time.Time) ([]*clientmodels.Message, error) {
	result, err := c.Client.ListChatMessages(chatID, since)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) ListSubscriptions() ([]*clientmodels.Subscription, error) {
	result, err := c.Client.ListSubscriptions()
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) ListTeams() ([]clientmodels.Team, error) {
	result, err := c.Client.ListTeams()
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) ListUsers() ([]clientmodels.User, error) {
	result, err := c.Client.ListUsers()
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) RefreshSubscription(subscriptionID string) (*time.Time, error) {
	result, err := c.Client.RefreshSubscription(subscriptionID)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) RefreshToken(token *oauth2.Token) (*oauth2.Token, error) {
	result, err := c.Client.RefreshToken(token)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) SendChat(chatID string, message string, parentMessage *clientmodels.Message, attachments []*clientmodels.Attachment, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	result, err := c.Client.SendChat(chatID, message, parentMessage, attachments, mentions)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) SendMessage(teamID string, channelID string, parentID string, message string) (*clientmodels.Message, error) {
	result, err := c.Client.SendMessage(teamID, channelID, parentID, message)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) SendMessageWithAttachments(teamID string, channelID string, parentID string, message string, attachments []*clientmodels.Attachment, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	result, err := c.Client.SendMessageWithAttachments(teamID, channelID, parentID, message, attachments, mentions)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) SetChatReaction(chatID string, messageID string, userID string, emoji string) (*clientmodels.Message, error) {
	result, err := c.Client.SetChatReaction(chatID, messageID, userID, emoji)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) SetReaction(teamID string, channelID string, parentID string, messageID string, userID string, emoji string) (*clientmodels.Message, error) {
	result, err := c.Client.SetReaction(teamID, channelID, parentID, messageID, userID, emoji)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) SubscribeToChannel(teamID string, channelID string, baseURL string, webhookSecret string, certificate string) (*clientmodels.Subscription, error) {
	result, err := c.Client.SubscribeToChannel(teamID, channelID, baseURL, webhookSecret, certificate)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) SubscribeToChannels(baseURL string, webhookSecret string, pay bool, certificate string) (*clientmodels.Subscription, error) {
	result, err := c.Client.SubscribeToChannels(baseURL, webhookSecret, pay, certificate)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) SubscribeToChats(baseURL string, webhookSecret string, pay bool, certificate string) (*clientmodels.Subscription, error) {
	result, err := c.Client.SubscribeToChats(baseURL, webhookSecret, pay, certificate)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) SubscribeToUserChats(user string, baseURL string, webhookSecret string, pay bool, certificate string) (*clientmodels.Subscription, error) {
	result, err := c.Client.SubscribeToUserChats(user, baseURL, webhookSecret, pay, certificate)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) UnsetChatReaction(chatID string, messageID string, userID string, emoji string) (*clientmodels.Message, error) {
	result, err := c.Client.UnsetChatReaction(chatID, messageID, userID, emoji)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) UnsetReaction(teamID string, channelID string, parentID string, messageID string, userID string, emoji string) (*clientmodels.Message, error) {
	result, err := c.Client.UnsetReaction(teamID, channelID, parentID, messageID, userID, emoji)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) UpdateChatMessage(chatID string, msgID string, message string, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	result, err := c.Client.UpdateChatMessage(chatID, msgID, message, mentions)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) UpdateMessage(teamID string, channelID string, parentID string, msgID string, message string, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	result, err := c.Client.UpdateMessage(teamID, channelID, parentID, msgID, message, mentions)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func (c *ClientDisconnectionLayer) UploadFile(teamID string, channelID string, filename string, filesize int, mimeType string, data io.Reader, chat *clientmodels.Chat) (*clientmodels.Attachment, error) {
	result, err := c.Client.UploadFile(teamID, channelID, filename, filesize, mimeType, data, chat)
	if err != nil {
		var graphErr *msteams.GraphAPIError
		if msteams.IsOAuthError(err) || (errors.As(err, &graphErr) && graphErr.StatusCode == http.StatusUnauthorized) {
			c.onDisconnect(c.userID)
		}
	}
	return result, err
}

func New(childClient msteams.Client, userID string, onDisconnect func(userID string)) *ClientDisconnectionLayer {
	return &ClientDisconnectionLayer{
		Client:       childClient,
		userID:       userID,
		onDisconnect: onDisconnect,
	}
}
