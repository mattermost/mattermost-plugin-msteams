package msteams

import (
	"io"

	"golang.org/x/oauth2"
)

type Client interface {
	Connect() error
	RequestUserToken(message chan string) (oauth2.TokenSource, error)
	CreateOrGetChatForUsers(usersIDs []string) (string, error)
	SendMessage(teamID, channelID, parentID, message string) (*Message, error)
	SendMessageWithAttachments(teamID, channelID, parentID, message string, attachments []*Attachment) (*Message, error)
	SendChat(chatID, parentID, message string) (*Message, error)
	UploadFile(teamID, channelID, filename string, filesize int, mimeType string, data io.Reader) (*Attachment, error)
	UpdateMessage(teamID, channelID, parentID, msgID, message string) error
	UpdateChatMessage(chatID, msgID, message string) error
	DeleteMessage(teamID, channelID, parentID, msgID string) error
	DeleteChatMessage(chatID, msgID string) error
	SubscribeToChannels(baseURL, webhookSecret string, pay bool) (*Subscription, error)
	SubscribeToChats(baseURL, webhookSecret string, pay bool) (*Subscription, error)
	SubscribeToChannel(teamID, channelID, baseURL, webhookSecret string) (*Subscription, error)
	SubscribeToUserChats(user, baseURL, webhookSecret string, pay bool) (*Subscription, error)
	RefreshSubscription(subscriptionID string) error
	DeleteSubscription(subscriptionID string) error
	GetTeam(teamID string) (*Team, error)
	GetChannel(teamID, channelID string) (*Channel, error)
	GetChat(chatID string) (*Chat, error)
	GetChatMessage(chatID, messageID string) (*Message, error)
	SetChatReaction(chatID, messageID, userID, emoji string) error
	SetReaction(teamID, channelID, parentID, messageID, userID, emoji string) error
	UnsetChatReaction(chatID, messageID, userID, emoji string) error
	UnsetReaction(teamID, channelID, parentID, messageID, userID, emoji string) error
	GetMessage(teamID, channelID, messageID string) (*Message, error)
	GetReply(teamID, channelID, messageID, replyID string) (*Message, error)
	GetUserAvatar(userID string) ([]byte, error)
	GetMyID() (string, error)
	GetFileURL(weburl string) (string, error)
	GetCodeSnippet(url string) (string, error)
	ListUsers() ([]User, error)
	ListTeams() ([]Team, error)
	ListChannels(teamID string) ([]Channel, error)
}
