//go:generate mockery --name=Client
//go:generate go run layer_generators/main.go

// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.
package msteams

import (
	"io"
	"net/url"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"golang.org/x/oauth2"
)

type Client interface {
	Connect() error
	CreateOrGetChatForUsers(usersIDs []string) (*clientmodels.Chat, error)
	SendMessage(teamID, channelID, parentID, message string) (*clientmodels.Message, error)
	SendMessageWithAttachments(teamID, channelID, parentID, message string, attachments []*clientmodels.Attachment, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error)
	SendChat(chatID, message string, parentMessage *clientmodels.Message, attachments []*clientmodels.Attachment, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error)
	UploadFile(teamID, channelID, filename string, filesize int, mimeType string, data io.Reader, chat *clientmodels.Chat) (*clientmodels.Attachment, error)
	UpdateMessage(teamID, channelID, parentID, msgID, message string, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error)
	UpdateChatMessage(chatID, msgID, message string, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error)
	DeleteMessage(teamID, channelID, parentID, msgID string) error
	DeleteChatMessage(userID, chatID, msgID string) error
	SubscribeToChannels(baseURL, webhookSecret string, pay bool, certificate string) (*clientmodels.Subscription, error)
	SubscribeToChats(baseURL, webhookSecret string, pay bool, certificate string) (*clientmodels.Subscription, error)
	SubscribeToChannel(teamID, channelID, baseURL, webhookSecret, certificate string) (*clientmodels.Subscription, error)
	SubscribeToUserChats(user, baseURL, webhookSecret string, pay bool, certificate string) (*clientmodels.Subscription, error)
	RefreshSubscription(subscriptionID string) (*time.Time, error)
	DeleteSubscription(subscriptionID string) error
	ListSubscriptions() ([]*clientmodels.Subscription, error)
	GetTeam(teamID string) (*clientmodels.Team, error)
	GetTeams(filterQuery string) ([]*clientmodels.Team, error)
	GetChannelInTeam(teamID, channelID string) (*clientmodels.Channel, error)
	GetChannelsInTeam(teamID, filterQuery string) ([]*clientmodels.Channel, error)
	GetChat(chatID string) (*clientmodels.Chat, error)
	GetChatMessage(chatID, messageID string) (*clientmodels.Message, error)
	SetChatReaction(chatID, messageID, userID, emoji string) (*clientmodels.Message, error)
	SetReaction(teamID, channelID, parentID, messageID, userID, emoji string) (*clientmodels.Message, error)
	UnsetChatReaction(chatID, messageID, userID, emoji string) (*clientmodels.Message, error)
	UnsetReaction(teamID, channelID, parentID, messageID, userID, emoji string) (*clientmodels.Message, error)
	GetMessage(teamID, channelID, messageID string) (*clientmodels.Message, error)
	GetReply(teamID, channelID, messageID, replyID string) (*clientmodels.Message, error)
	GetUserAvatar(userID string) ([]byte, error)
	GetUser(userID string) (*clientmodels.User, error)
	GetMyID() (string, error)
	GetMe() (*clientmodels.User, error)
	GetFileSizeAndDownloadURL(weburl string) (int64, string, error)
	GetFileContent(downloadURL string) ([]byte, error)
	GetFileContentStream(downloadURL string, writer *io.PipeWriter, bufferSize int64)
	GetHostedFileContent(activityIDs *clientmodels.ActivityIds) ([]byte, error)
	GetCodeSnippet(url string) (string, error)
	RefreshToken(token *oauth2.Token) (*oauth2.Token, error)
	ListUsers() ([]clientmodels.User, error)
	ListTeams() ([]clientmodels.Team, error)
	ListChannels(teamID string) ([]clientmodels.Channel, error)
	ListChannelMessages(teamID, channelID string, since time.Time) ([]*clientmodels.Message, error)
	ListChatMessages(chatID string, since time.Time) ([]*clientmodels.Message, error)
	GetApp(applicationID string) (*clientmodels.App, error)
	GetPresencesForUsers(userIDs []string) (map[string]clientmodels.Presence, error)
	SendUserActivity(userID, activityType, message string, urlParams url.Values, params map[string]string) error
}
