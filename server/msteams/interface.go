package msteams

import (
	"context"
)

type Client interface {
	Connect() error
	SendMessage(teamID, channelID, parentID, message string) (string, error)
	SubscribeToChannel(teamID, channelID, notificationURL, webhookSecret string) (string, error)
	RefreshSubscriptionPeriodically(ctx context.Context, subscriptionID string) error
	ClearSubscriptions() error
	GetTeam(teamID string) (*Team, error)
	GetChannel(teamID, channelID string) (*Channel, error)
	GetMessage(teamID, channelID, messageID string) (*Message, error)
	GetReply(teamID, channelID, messageID, replyID string) (*Message, error)
	GetUserAvatar(userID string) ([]byte, error)
	GetFileURL(weburl string) (string, error)
	GetCodeSnippet(url string) (string, error)
}
