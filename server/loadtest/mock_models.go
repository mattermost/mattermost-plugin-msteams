package loadtest

import "time"

type MSContentFromUser struct {
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	UserIdentityType string `json:"userIdentityType"`
}

type MSContentFrom struct {
	Application  any               `json:"application"`
	Device       any               `json:"device"`
	User         MSContentFromUser `json:"user"`
	Conversation any               `json:"conversation"`
}

type MSContentBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type MSContent struct {
	ID                   string        `json:"id"`
	ReplyToID            any           `json:"replyToId"`
	Etag                 string        `json:"etag"`
	MessageType          string        `json:"messageType"`
	CreatedDateTime      time.Time     `json:"createdDateTime"`
	LastModifiedDateTime time.Time     `json:"lastModifiedDateTime"`
	LastEditedDateTime   any           `json:"lastEditedDateTime"`
	DeletedDateTime      any           `json:"deletedDateTime"`
	Subject              any           `json:"subject"`
	Summary              any           `json:"summary"`
	ChatID               string        `json:"chatId"`
	Importance           string        `json:"importance"`
	Locale               string        `json:"locale"`
	WebURL               any           `json:"webUrl"`
	From                 MSContentFrom `json:"from"`
	Body                 MSContentBody `json:"body"`
	ChannelIdentity      any           `json:"channelIdentity"`
	Attachments          []any         `json:"attachments"`
	Mentions             []any         `json:"mentions"`
	PolicyViolation      any           `json:"policyViolation"`
	Reactions            []any         `json:"reactions"`
	Replies              []any         `json:"replies"`
	HostedContents       []any         `json:"hostedContents"`
}

type MSActivity struct {
	SubscriptionID                 string `json:"subscriptionId"`
	ChangeType                     string `json:"changeType"`
	ClientState                    string `json:"clientState"`
	SubscriptionExpirationDateTime string `json:"subscriptionExpirationDateTime"`
	Resource                       string `json:"resource"`
	Content                        []byte `json:"content"`
	TenantID                       string `json:"tenantId"`
}

type MSActivities struct {
	Value []MSActivity `json:"value"`
}
