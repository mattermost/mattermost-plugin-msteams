package msteams

import (
	"io"
	"time"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

type Client interface {
	Connect() error
	CreateOrGetChatForUsers(usersIDs []string) (*Chat, error)
	SendMessage(teamID, channelID, parentID, message string) (*Message, error)
	SendMessageWithAttachments(teamID, channelID, parentID, message string, attachments []*Attachment, mentions []models.ChatMessageMentionable) (*Message, error)
	SendChat(chatID, message string, parentMessage *Message, attachments []*Attachment, mentions []models.ChatMessageMentionable) (*Message, error)
	UploadFile(teamID, channelID, filename string, filesize int, mimeType string, data io.Reader, chat *Chat) (*Attachment, error)
	UpdateMessage(teamID, channelID, parentID, msgID, message string, mentions []models.ChatMessageMentionable) error
	UpdateChatMessage(chatID, msgID, message string, mentions []models.ChatMessageMentionable) error
	DeleteMessage(teamID, channelID, parentID, msgID string) error
	DeleteChatMessage(chatID, msgID string) error
	SubscribeToChannels(baseURL, webhookSecret string, pay bool) (*Subscription, error)
	SubscribeToChats(baseURL, webhookSecret string, pay bool) (*Subscription, error)
	SubscribeToChannel(teamID, channelID, baseURL, webhookSecret string) (*Subscription, error)
	SubscribeToUserChats(user, baseURL, webhookSecret string, pay bool) (*Subscription, error)
	RefreshSubscription(subscriptionID string) (*time.Time, error)
	DeleteSubscription(subscriptionID string) error
	GetSubscription(subscriptionID string) (*Subscription, error)
	GetTeam(teamID string) (*Team, error)
	GetTeams(filterQuery string) ([]*Team, error)
	GetChannelInTeam(teamID, channelID string) (*Channel, error)
	GetChannelsInTeam(teamID, filterQuery string) ([]*Channel, error)
	GetChat(chatID string) (*Chat, error)
	GetChatMessage(chatID, messageID string) (*Message, error)
	SetChatReaction(chatID, messageID, userID, emoji string) error
	SetReaction(teamID, channelID, parentID, messageID, userID, emoji string) error
	UnsetChatReaction(chatID, messageID, userID, emoji string) error
	UnsetReaction(teamID, channelID, parentID, messageID, userID, emoji string) error
	GetMessage(teamID, channelID, messageID string) (*Message, error)
	GetReply(teamID, channelID, messageID, replyID string) (*Message, error)
	GetUserAvatar(userID string) ([]byte, error)
	GetUser(userID string) (*User, error)
	GetMyID() (string, error)
	GetMe() (*User, error)
	GetFileContent(weburl string) ([]byte, error)
	GetHostedFileContent(activityIDs *ActivityIds) ([]byte, error)
	GetCodeSnippet(url string) (string, error)
	ListUsers() ([]User, error)
	ListTeams() ([]Team, error)
	ListChannels(teamID string) ([]Channel, error)
}
