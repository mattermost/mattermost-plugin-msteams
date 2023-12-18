// Code generated by mockery v2.18.0. DO NOT EDIT.

package mocks

import (
	io "io"

	clientmodels "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"

	mock "github.com/stretchr/testify/mock"

	models "github.com/microsoftgraph/msgraph-sdk-go/models"

	time "time"
)

// Client is an autogenerated mock type for the Client type
type Client struct {
	mock.Mock
}

// Connect provides a mock function with given fields:
func (_m *Client) Connect() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateOrGetChatForUsers provides a mock function with given fields: usersIDs
func (_m *Client) CreateOrGetChatForUsers(usersIDs []string) (*clientmodels.Chat, error) {
	ret := _m.Called(usersIDs)

	var r0 *clientmodels.Chat
	if rf, ok := ret.Get(0).(func([]string) *clientmodels.Chat); ok {
		r0 = rf(usersIDs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Chat)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func([]string) error); ok {
		r1 = rf(usersIDs)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteChatMessage provides a mock function with given fields: chatID, msgID
func (_m *Client) DeleteChatMessage(chatID string, msgID string) error {
	ret := _m.Called(chatID, msgID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string) error); ok {
		r0 = rf(chatID, msgID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteMessage provides a mock function with given fields: teamID, channelID, parentID, msgID
func (_m *Client) DeleteMessage(teamID string, channelID string, parentID string, msgID string) error {
	ret := _m.Called(teamID, channelID, parentID, msgID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, string, string) error); ok {
		r0 = rf(teamID, channelID, parentID, msgID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteSubscription provides a mock function with given fields: subscriptionID
func (_m *Client) DeleteSubscription(subscriptionID string) error {
	ret := _m.Called(subscriptionID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(subscriptionID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetChannelInTeam provides a mock function with given fields: teamID, channelID
func (_m *Client) GetChannelInTeam(teamID string, channelID string) (*clientmodels.Channel, error) {
	ret := _m.Called(teamID, channelID)

	var r0 *clientmodels.Channel
	if rf, ok := ret.Get(0).(func(string, string) *clientmodels.Channel); ok {
		r0 = rf(teamID, channelID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Channel)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(teamID, channelID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetChannelsInTeam provides a mock function with given fields: teamID, filterQuery
func (_m *Client) GetChannelsInTeam(teamID string, filterQuery string) ([]*clientmodels.Channel, error) {
	ret := _m.Called(teamID, filterQuery)

	var r0 []*clientmodels.Channel
	if rf, ok := ret.Get(0).(func(string, string) []*clientmodels.Channel); ok {
		r0 = rf(teamID, filterQuery)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*clientmodels.Channel)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(teamID, filterQuery)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetChat provides a mock function with given fields: chatID
func (_m *Client) GetChat(chatID string) (*clientmodels.Chat, error) {
	ret := _m.Called(chatID)

	var r0 *clientmodels.Chat
	if rf, ok := ret.Get(0).(func(string) *clientmodels.Chat); ok {
		r0 = rf(chatID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Chat)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(chatID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetChatMessage provides a mock function with given fields: chatID, messageID
func (_m *Client) GetChatMessage(chatID string, messageID string) (*clientmodels.Message, error) {
	ret := _m.Called(chatID, messageID)

	var r0 *clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, string) *clientmodels.Message); ok {
		r0 = rf(chatID, messageID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(chatID, messageID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCodeSnippet provides a mock function with given fields: url
func (_m *Client) GetCodeSnippet(url string) (string, error) {
	ret := _m.Called(url)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(url)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(url)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetFileContent provides a mock function with given fields: downloadURL
func (_m *Client) GetFileContent(downloadURL string) ([]byte, error) {
	ret := _m.Called(downloadURL)

	var r0 []byte
	if rf, ok := ret.Get(0).(func(string) []byte); ok {
		r0 = rf(downloadURL)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(downloadURL)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetFileContentStream provides a mock function with given fields: downloadURL, writer, bufferSize
func (_m *Client) GetFileContentStream(downloadURL string, writer *io.PipeWriter, bufferSize int64) {
	_m.Called(downloadURL, writer, bufferSize)
}

// GetFileSizeAndDownloadURL provides a mock function with given fields: weburl
func (_m *Client) GetFileSizeAndDownloadURL(weburl string) (int64, string, error) {
	ret := _m.Called(weburl)

	var r0 int64
	if rf, ok := ret.Get(0).(func(string) int64); ok {
		r0 = rf(weburl)
	} else {
		r0 = ret.Get(0).(int64)
	}

	var r1 string
	if rf, ok := ret.Get(1).(func(string) string); ok {
		r1 = rf(weburl)
	} else {
		r1 = ret.Get(1).(string)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string) error); ok {
		r2 = rf(weburl)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetHostedFileContent provides a mock function with given fields: activityIDs
func (_m *Client) GetHostedFileContent(activityIDs *clientmodels.ActivityIds) ([]byte, error) {
	ret := _m.Called(activityIDs)

	var r0 []byte
	if rf, ok := ret.Get(0).(func(*clientmodels.ActivityIds) []byte); ok {
		r0 = rf(activityIDs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*clientmodels.ActivityIds) error); ok {
		r1 = rf(activityIDs)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetMe provides a mock function with given fields:
func (_m *Client) GetMe() (*clientmodels.User, error) {
	ret := _m.Called()

	var r0 *clientmodels.User
	if rf, ok := ret.Get(0).(func() *clientmodels.User); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.User)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetMessage provides a mock function with given fields: teamID, channelID, messageID
func (_m *Client) GetMessage(teamID string, channelID string, messageID string) (*clientmodels.Message, error) {
	ret := _m.Called(teamID, channelID, messageID)

	var r0 *clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, string, string) *clientmodels.Message); ok {
		r0 = rf(teamID, channelID, messageID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string) error); ok {
		r1 = rf(teamID, channelID, messageID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetMyID provides a mock function with given fields:
func (_m *Client) GetMyID() (string, error) {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetReply provides a mock function with given fields: teamID, channelID, messageID, replyID
func (_m *Client) GetReply(teamID string, channelID string, messageID string, replyID string) (*clientmodels.Message, error) {
	ret := _m.Called(teamID, channelID, messageID, replyID)

	var r0 *clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, string, string, string) *clientmodels.Message); ok {
		r0 = rf(teamID, channelID, messageID, replyID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string) error); ok {
		r1 = rf(teamID, channelID, messageID, replyID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTeam provides a mock function with given fields: teamID
func (_m *Client) GetTeam(teamID string) (*clientmodels.Team, error) {
	ret := _m.Called(teamID)

	var r0 *clientmodels.Team
	if rf, ok := ret.Get(0).(func(string) *clientmodels.Team); ok {
		r0 = rf(teamID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Team)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(teamID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTeams provides a mock function with given fields: filterQuery
func (_m *Client) GetTeams(filterQuery string) ([]*clientmodels.Team, error) {
	ret := _m.Called(filterQuery)

	var r0 []*clientmodels.Team
	if rf, ok := ret.Get(0).(func(string) []*clientmodels.Team); ok {
		r0 = rf(filterQuery)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*clientmodels.Team)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(filterQuery)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetUser provides a mock function with given fields: userID
func (_m *Client) GetUser(userID string) (*clientmodels.User, error) {
	ret := _m.Called(userID)

	var r0 *clientmodels.User
	if rf, ok := ret.Get(0).(func(string) *clientmodels.User); ok {
		r0 = rf(userID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.User)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(userID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetUserAvatar provides a mock function with given fields: userID
func (_m *Client) GetUserAvatar(userID string) ([]byte, error) {
	ret := _m.Called(userID)

	var r0 []byte
	if rf, ok := ret.Get(0).(func(string) []byte); ok {
		r0 = rf(userID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(userID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListChannels provides a mock function with given fields: teamID
func (_m *Client) ListChannels(teamID string) ([]clientmodels.Channel, error) {
	ret := _m.Called(teamID)

	var r0 []clientmodels.Channel
	if rf, ok := ret.Get(0).(func(string) []clientmodels.Channel); ok {
		r0 = rf(teamID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]clientmodels.Channel)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(teamID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListChatMessages provides a mock function with given fields: chatID, since
func (_m *Client) ListChatMessages(chatID string, since time.Time) ([]*clientmodels.Message, error) {
	ret := _m.Called(chatID, since)

	var r0 []*clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, time.Time) []*clientmodels.Message); ok {
		r0 = rf(chatID, since)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, time.Time) error); ok {
		r1 = rf(chatID, since)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListSubscriptions provides a mock function with given fields:
func (_m *Client) ListSubscriptions() ([]*clientmodels.Subscription, error) {
	ret := _m.Called()

	var r0 []*clientmodels.Subscription
	if rf, ok := ret.Get(0).(func() []*clientmodels.Subscription); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*clientmodels.Subscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListTeams provides a mock function with given fields:
func (_m *Client) ListTeams() ([]clientmodels.Team, error) {
	ret := _m.Called()

	var r0 []clientmodels.Team
	if rf, ok := ret.Get(0).(func() []clientmodels.Team); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]clientmodels.Team)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListUsers provides a mock function with given fields:
func (_m *Client) ListUsers() ([]clientmodels.User, error) {
	ret := _m.Called()

	var r0 []clientmodels.User
	if rf, ok := ret.Get(0).(func() []clientmodels.User); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]clientmodels.User)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// OnChannelMessagesSince provides a mock function with given fields: teamID, channelID, since, callback
func (_m *Client) OnChannelMessagesSince(teamID string, channelID string, since time.Time, callback func(*clientmodels.Message)) error {
	ret := _m.Called(teamID, channelID, since, callback)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, time.Time, func(*clientmodels.Message)) error); ok {
		r0 = rf(teamID, channelID, since, callback)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RefreshSubscription provides a mock function with given fields: subscriptionID
func (_m *Client) RefreshSubscription(subscriptionID string) (*time.Time, error) {
	ret := _m.Called(subscriptionID)

	var r0 *time.Time
	if rf, ok := ret.Get(0).(func(string) *time.Time); ok {
		r0 = rf(subscriptionID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*time.Time)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(subscriptionID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SendChat provides a mock function with given fields: chatID, message, parentMessage, attachments, mentions
func (_m *Client) SendChat(chatID string, message string, parentMessage *clientmodels.Message, attachments []*clientmodels.Attachment, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	ret := _m.Called(chatID, message, parentMessage, attachments, mentions)

	var r0 *clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, string, *clientmodels.Message, []*clientmodels.Attachment, []models.ChatMessageMentionable) *clientmodels.Message); ok {
		r0 = rf(chatID, message, parentMessage, attachments, mentions)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, *clientmodels.Message, []*clientmodels.Attachment, []models.ChatMessageMentionable) error); ok {
		r1 = rf(chatID, message, parentMessage, attachments, mentions)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SendMessage provides a mock function with given fields: teamID, channelID, parentID, message
func (_m *Client) SendMessage(teamID string, channelID string, parentID string, message string) (*clientmodels.Message, error) {
	ret := _m.Called(teamID, channelID, parentID, message)

	var r0 *clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, string, string, string) *clientmodels.Message); ok {
		r0 = rf(teamID, channelID, parentID, message)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string) error); ok {
		r1 = rf(teamID, channelID, parentID, message)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SendMessageWithAttachments provides a mock function with given fields: teamID, channelID, parentID, message, attachments, mentions
func (_m *Client) SendMessageWithAttachments(teamID string, channelID string, parentID string, message string, attachments []*clientmodels.Attachment, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	ret := _m.Called(teamID, channelID, parentID, message, attachments, mentions)

	var r0 *clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, string, string, string, []*clientmodels.Attachment, []models.ChatMessageMentionable) *clientmodels.Message); ok {
		r0 = rf(teamID, channelID, parentID, message, attachments, mentions)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string, []*clientmodels.Attachment, []models.ChatMessageMentionable) error); ok {
		r1 = rf(teamID, channelID, parentID, message, attachments, mentions)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SetChatReaction provides a mock function with given fields: chatID, messageID, userID, emoji
func (_m *Client) SetChatReaction(chatID string, messageID string, userID string, emoji string) (*clientmodels.Message, error) {
	ret := _m.Called(chatID, messageID, userID, emoji)

	var r0 *clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, string, string, string) *clientmodels.Message); ok {
		r0 = rf(chatID, messageID, userID, emoji)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string) error); ok {
		r1 = rf(chatID, messageID, userID, emoji)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SetReaction provides a mock function with given fields: teamID, channelID, parentID, messageID, userID, emoji
func (_m *Client) SetReaction(teamID string, channelID string, parentID string, messageID string, userID string, emoji string) (*clientmodels.Message, error) {
	ret := _m.Called(teamID, channelID, parentID, messageID, userID, emoji)

	var r0 *clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, string, string, string, string, string) *clientmodels.Message); ok {
		r0 = rf(teamID, channelID, parentID, messageID, userID, emoji)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string, string, string) error); ok {
		r1 = rf(teamID, channelID, parentID, messageID, userID, emoji)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SubscribeToChannel provides a mock function with given fields: teamID, channelID, baseURL, webhookSecret, certificate
func (_m *Client) SubscribeToChannel(teamID string, channelID string, baseURL string, webhookSecret string, certificate string) (*clientmodels.Subscription, error) {
	ret := _m.Called(teamID, channelID, baseURL, webhookSecret, certificate)

	var r0 *clientmodels.Subscription
	if rf, ok := ret.Get(0).(func(string, string, string, string, string) *clientmodels.Subscription); ok {
		r0 = rf(teamID, channelID, baseURL, webhookSecret, certificate)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Subscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string, string) error); ok {
		r1 = rf(teamID, channelID, baseURL, webhookSecret, certificate)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SubscribeToChannels provides a mock function with given fields: baseURL, webhookSecret, pay, certificate
func (_m *Client) SubscribeToChannels(baseURL string, webhookSecret string, pay bool, certificate string) (*clientmodels.Subscription, error) {
	ret := _m.Called(baseURL, webhookSecret, pay, certificate)

	var r0 *clientmodels.Subscription
	if rf, ok := ret.Get(0).(func(string, string, bool, string) *clientmodels.Subscription); ok {
		r0 = rf(baseURL, webhookSecret, pay, certificate)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Subscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, bool, string) error); ok {
		r1 = rf(baseURL, webhookSecret, pay, certificate)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SubscribeToChats provides a mock function with given fields: baseURL, webhookSecret, pay, certificate
func (_m *Client) SubscribeToChats(baseURL string, webhookSecret string, pay bool, certificate string) (*clientmodels.Subscription, error) {
	ret := _m.Called(baseURL, webhookSecret, pay, certificate)

	var r0 *clientmodels.Subscription
	if rf, ok := ret.Get(0).(func(string, string, bool, string) *clientmodels.Subscription); ok {
		r0 = rf(baseURL, webhookSecret, pay, certificate)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Subscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, bool, string) error); ok {
		r1 = rf(baseURL, webhookSecret, pay, certificate)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SubscribeToUserChats provides a mock function with given fields: user, baseURL, webhookSecret, pay, certificate
func (_m *Client) SubscribeToUserChats(user string, baseURL string, webhookSecret string, pay bool, certificate string) (*clientmodels.Subscription, error) {
	ret := _m.Called(user, baseURL, webhookSecret, pay, certificate)

	var r0 *clientmodels.Subscription
	if rf, ok := ret.Get(0).(func(string, string, string, bool, string) *clientmodels.Subscription); ok {
		r0 = rf(user, baseURL, webhookSecret, pay, certificate)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Subscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, bool, string) error); ok {
		r1 = rf(user, baseURL, webhookSecret, pay, certificate)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UnsetChatReaction provides a mock function with given fields: chatID, messageID, userID, emoji
func (_m *Client) UnsetChatReaction(chatID string, messageID string, userID string, emoji string) (*clientmodels.Message, error) {
	ret := _m.Called(chatID, messageID, userID, emoji)

	var r0 *clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, string, string, string) *clientmodels.Message); ok {
		r0 = rf(chatID, messageID, userID, emoji)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string) error); ok {
		r1 = rf(chatID, messageID, userID, emoji)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UnsetReaction provides a mock function with given fields: teamID, channelID, parentID, messageID, userID, emoji
func (_m *Client) UnsetReaction(teamID string, channelID string, parentID string, messageID string, userID string, emoji string) (*clientmodels.Message, error) {
	ret := _m.Called(teamID, channelID, parentID, messageID, userID, emoji)

	var r0 *clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, string, string, string, string, string) *clientmodels.Message); ok {
		r0 = rf(teamID, channelID, parentID, messageID, userID, emoji)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string, string, string) error); ok {
		r1 = rf(teamID, channelID, parentID, messageID, userID, emoji)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateChatMessage provides a mock function with given fields: chatID, msgID, message, mentions
func (_m *Client) UpdateChatMessage(chatID string, msgID string, message string, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	ret := _m.Called(chatID, msgID, message, mentions)

	var r0 *clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, string, string, []models.ChatMessageMentionable) *clientmodels.Message); ok {
		r0 = rf(chatID, msgID, message, mentions)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, []models.ChatMessageMentionable) error); ok {
		r1 = rf(chatID, msgID, message, mentions)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateMessage provides a mock function with given fields: teamID, channelID, parentID, msgID, message, mentions
func (_m *Client) UpdateMessage(teamID string, channelID string, parentID string, msgID string, message string, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	ret := _m.Called(teamID, channelID, parentID, msgID, message, mentions)

	var r0 *clientmodels.Message
	if rf, ok := ret.Get(0).(func(string, string, string, string, string, []models.ChatMessageMentionable) *clientmodels.Message); ok {
		r0 = rf(teamID, channelID, parentID, msgID, message, mentions)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string, string, []models.ChatMessageMentionable) error); ok {
		r1 = rf(teamID, channelID, parentID, msgID, message, mentions)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UploadFile provides a mock function with given fields: teamID, channelID, filename, filesize, mimeType, data, chat
func (_m *Client) UploadFile(teamID string, channelID string, filename string, filesize int, mimeType string, data io.Reader, chat *clientmodels.Chat) (*clientmodels.Attachment, error) {
	ret := _m.Called(teamID, channelID, filename, filesize, mimeType, data, chat)

	var r0 *clientmodels.Attachment
	if rf, ok := ret.Get(0).(func(string, string, string, int, string, io.Reader, *clientmodels.Chat) *clientmodels.Attachment); ok {
		r0 = rf(teamID, channelID, filename, filesize, mimeType, data, chat)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientmodels.Attachment)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, int, string, io.Reader, *clientmodels.Chat) error); ok {
		r1 = rf(teamID, channelID, filename, filesize, mimeType, data, chat)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewClient interface {
	mock.TestingT
	Cleanup(func())
}

// NewClient creates a new instance of Client. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewClient(t mockConstructorTestingTNewClient) *Client {
	mock := &Client{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
