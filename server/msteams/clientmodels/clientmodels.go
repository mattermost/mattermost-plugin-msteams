package clientmodels

import (
	"io"
	"time"
)

type Chat struct {
	ID      string
	Members []ChatMember
	Type    string
	Topic   string
}

type ChatMember struct {
	DisplayName string
	UserID      string
	Email       string
}

type Attachment struct {
	ID           string
	ContentType  string
	Content      string
	Name         string
	ContentURL   string
	ThumbnailURL string
	Data         io.Reader
}

type Reaction struct {
	UserID   string
	Reaction string
}

type Mention struct {
	ID             int32
	UserID         string
	MentionedText  string
	ConversationID string
}

type Message struct {
	ID              string
	UserID          string
	UserDisplayName string
	Text            string
	Subject         string
	ReplyToID       string
	Attachments     []Attachment
	Reactions       []Reaction
	Mentions        []Mention
	ChannelID       string
	TeamID          string
	ChatID          string
	CreateAt        time.Time
	LastUpdateAt    time.Time
}

type Subscription struct {
	ID              string
	Type            string
	ChannelID       string
	Resource        string
	TeamID          string
	UserID          string
	ExpiresOn       time.Time
	NotificationURL string
}

type Channel struct {
	ID          string
	DisplayName string
	Description string
}

type User struct {
	DisplayName       string
	ID                string
	Mail              string
	UserPrincipalName string
	Type              string
	IsAccountEnabled  bool
}

type Team struct {
	ID          string
	DisplayName string
	Description string
}

type ActivityIds struct {
	ChatID           string
	TeamID           string
	ChannelID        string
	MessageID        string
	ReplyID          string
	HostedContentsID string
}

type Credential struct {
	Name        string
	ID          string
	EndDateTime time.Time
	Hint        string
}

type App struct {
	Credentials []Credential
}
