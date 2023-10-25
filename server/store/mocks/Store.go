// Code generated by mockery v2.18.0. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	oauth2 "golang.org/x/oauth2"

	sql "database/sql"

	storemodels "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"

	time "time"
)

// Store is an autogenerated mock type for the Store type
type Store struct {
	mock.Mock
}

// BeginTx provides a mock function with given fields:
func (_m *Store) BeginTx() (*sql.Tx, error) {
	ret := _m.Called()

	var r0 *sql.Tx
	if rf, ok := ret.Get(0).(func() *sql.Tx); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*sql.Tx)
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

// CheckEnabledTeamByTeamID provides a mock function with given fields: teamID
func (_m *Store) CheckEnabledTeamByTeamID(teamID string) bool {
	ret := _m.Called(teamID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string) bool); ok {
		r0 = rf(teamID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// CommitTx provides a mock function with given fields: tx
func (_m *Store) CommitTx(tx *sql.Tx) error {
	ret := _m.Called(tx)

	var r0 error
	if rf, ok := ret.Get(0).(func(*sql.Tx) error); ok {
		r0 = rf(tx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CompareAndSetJobStatus provides a mock function with given fields: jobName, oldStatus, newStatus
func (_m *Store) CompareAndSetJobStatus(jobName string, oldStatus bool, newStatus bool) (bool, error) {
	ret := _m.Called(jobName, oldStatus, newStatus)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string, bool, bool) bool); ok {
		r0 = rf(jobName, oldStatus, newStatus)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, bool, bool) error); ok {
		r1 = rf(jobName, oldStatus, newStatus)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteDMAndGMChannelPromptTime provides a mock function with given fields: userID
func (_m *Store) DeleteDMAndGMChannelPromptTime(userID string) error {
	ret := _m.Called(userID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(userID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteLinkByChannelID provides a mock function with given fields: channelID
func (_m *Store) DeleteLinkByChannelID(channelID string) error {
	ret := _m.Called(channelID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(channelID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteSubscription provides a mock function with given fields: subscriptionID
func (_m *Store) DeleteSubscription(subscriptionID string) error {
	ret := _m.Called(subscriptionID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(subscriptionID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteUserInfo provides a mock function with given fields: mmUserID
func (_m *Store) DeleteUserInfo(mmUserID string) error {
	ret := _m.Called(mmUserID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(mmUserID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetAvatarCache provides a mock function with given fields: userID
func (_m *Store) GetAvatarCache(userID string) ([]byte, error) {
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

// GetChannelSubscription provides a mock function with given fields: subscriptionID
func (_m *Store) GetChannelSubscription(subscriptionID string) (*storemodels.ChannelSubscription, error) {
	ret := _m.Called(subscriptionID)

	var r0 *storemodels.ChannelSubscription
	if rf, ok := ret.Get(0).(func(string) *storemodels.ChannelSubscription); ok {
		r0 = rf(subscriptionID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*storemodels.ChannelSubscription)
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

// GetChannelSubscriptionByTeamsChannelID provides a mock function with given fields: teamsChannelID
func (_m *Store) GetChannelSubscriptionByTeamsChannelID(teamsChannelID string) (*storemodels.ChannelSubscription, error) {
	ret := _m.Called(teamsChannelID)

	var r0 *storemodels.ChannelSubscription
	if rf, ok := ret.Get(0).(func(string) *storemodels.ChannelSubscription); ok {
		r0 = rf(teamsChannelID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*storemodels.ChannelSubscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(teamsChannelID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetChatSubscription provides a mock function with given fields: subscriptionID
func (_m *Store) GetChatSubscription(subscriptionID string) (*storemodels.ChatSubscription, error) {
	ret := _m.Called(subscriptionID)

	var r0 *storemodels.ChatSubscription
	if rf, ok := ret.Get(0).(func(string) *storemodels.ChatSubscription); ok {
		r0 = rf(subscriptionID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*storemodels.ChatSubscription)
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

// GetConnectedUsers provides a mock function with given fields: page, perPage
func (_m *Store) GetConnectedUsers(page int, perPage int) ([]*storemodels.ConnectedUser, error) {
	ret := _m.Called(page, perPage)

	var r0 []*storemodels.ConnectedUser
	if rf, ok := ret.Get(0).(func(int, int) []*storemodels.ConnectedUser); ok {
		r0 = rf(page, perPage)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*storemodels.ConnectedUser)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(int, int) error); ok {
		r1 = rf(page, perPage)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetDMAndGMChannelPromptTime provides a mock function with given fields: channelID, userID
func (_m *Store) GetDMAndGMChannelPromptTime(channelID string, userID string) (time.Time, error) {
	ret := _m.Called(channelID, userID)

	var r0 time.Time
	if rf, ok := ret.Get(0).(func(string, string) time.Time); ok {
		r0 = rf(channelID, userID)
	} else {
		r0 = ret.Get(0).(time.Time)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(channelID, userID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetGlobalSubscription provides a mock function with given fields: subscriptionID
func (_m *Store) GetGlobalSubscription(subscriptionID string) (*storemodels.GlobalSubscription, error) {
	ret := _m.Called(subscriptionID)

	var r0 *storemodels.GlobalSubscription
	if rf, ok := ret.Get(0).(func(string) *storemodels.GlobalSubscription); ok {
		r0 = rf(subscriptionID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*storemodels.GlobalSubscription)
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

// GetLinkByChannelID provides a mock function with given fields: channelID
func (_m *Store) GetLinkByChannelID(channelID string) (*storemodels.ChannelLink, error) {
	ret := _m.Called(channelID)

	var r0 *storemodels.ChannelLink
	if rf, ok := ret.Get(0).(func(string) *storemodels.ChannelLink); ok {
		r0 = rf(channelID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*storemodels.ChannelLink)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(channelID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLinkByMSTeamsChannelID provides a mock function with given fields: teamID, channelID
func (_m *Store) GetLinkByMSTeamsChannelID(teamID string, channelID string) (*storemodels.ChannelLink, error) {
	ret := _m.Called(teamID, channelID)

	var r0 *storemodels.ChannelLink
	if rf, ok := ret.Get(0).(func(string, string) *storemodels.ChannelLink); ok {
		r0 = rf(teamID, channelID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*storemodels.ChannelLink)
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

// GetPostInfoByMSTeamsID provides a mock function with given fields: chatID, postID
func (_m *Store) GetPostInfoByMSTeamsID(chatID string, postID string) (*storemodels.PostInfo, error) {
	ret := _m.Called(chatID, postID)

	var r0 *storemodels.PostInfo
	if rf, ok := ret.Get(0).(func(string, string) *storemodels.PostInfo); ok {
		r0 = rf(chatID, postID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*storemodels.PostInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(chatID, postID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPostInfoByMattermostID provides a mock function with given fields: postID
func (_m *Store) GetPostInfoByMattermostID(postID string) (*storemodels.PostInfo, error) {
	ret := _m.Called(postID)

	var r0 *storemodels.PostInfo
	if rf, ok := ret.Get(0).(func(string) *storemodels.PostInfo); ok {
		r0 = rf(postID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*storemodels.PostInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(postID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetSizeOfWhitelist provides a mock function with given fields:
func (_m *Store) GetSizeOfWhitelist() (int, error) {
	ret := _m.Called()

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStats provides a mock function with given fields:
func (_m *Store) GetStats() (*storemodels.Stats, error) {
	ret := _m.Called()

	var r0 *storemodels.Stats
	if rf, ok := ret.Get(0).(func() *storemodels.Stats); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*storemodels.Stats)
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

// GetSubscriptionType provides a mock function with given fields: subscriptionID
func (_m *Store) GetSubscriptionType(subscriptionID string) (string, error) {
	ret := _m.Called(subscriptionID)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(subscriptionID)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(subscriptionID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTokenForMSTeamsUser provides a mock function with given fields: userID
func (_m *Store) GetTokenForMSTeamsUser(userID string) (*oauth2.Token, error) {
	ret := _m.Called(userID)

	var r0 *oauth2.Token
	if rf, ok := ret.Get(0).(func(string) *oauth2.Token); ok {
		r0 = rf(userID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*oauth2.Token)
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

// GetTokenForMattermostUser provides a mock function with given fields: userID
func (_m *Store) GetTokenForMattermostUser(userID string) (*oauth2.Token, error) {
	ret := _m.Called(userID)

	var r0 *oauth2.Token
	if rf, ok := ret.Get(0).(func(string) *oauth2.Token); ok {
		r0 = rf(userID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*oauth2.Token)
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

// Init provides a mock function with given fields:
func (_m *Store) Init() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// IsUserPresentInWhitelist provides a mock function with given fields: userID
func (_m *Store) IsUserPresentInWhitelist(userID string) (bool, error) {
	ret := _m.Called(userID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string) bool); ok {
		r0 = rf(userID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(userID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LinkPosts provides a mock function with given fields: tx, postInfo
func (_m *Store) LinkPosts(tx *sql.Tx, postInfo storemodels.PostInfo) error {
	ret := _m.Called(tx, postInfo)

	var r0 error
	if rf, ok := ret.Get(0).(func(*sql.Tx, storemodels.PostInfo) error); ok {
		r0 = rf(tx, postInfo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ListChannelLinks provides a mock function with given fields:
func (_m *Store) ListChannelLinks() ([]storemodels.ChannelLink, error) {
	ret := _m.Called()

	var r0 []storemodels.ChannelLink
	if rf, ok := ret.Get(0).(func() []storemodels.ChannelLink); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]storemodels.ChannelLink)
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

// ListChannelLinksWithNames provides a mock function with given fields:
func (_m *Store) ListChannelLinksWithNames() ([]*storemodels.ChannelLink, error) {
	ret := _m.Called()

	var r0 []*storemodels.ChannelLink
	if rf, ok := ret.Get(0).(func() []*storemodels.ChannelLink); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*storemodels.ChannelLink)
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

// ListChannelSubscriptions provides a mock function with given fields:
func (_m *Store) ListChannelSubscriptions() ([]*storemodels.ChannelSubscription, error) {
	ret := _m.Called()

	var r0 []*storemodels.ChannelSubscription
	if rf, ok := ret.Get(0).(func() []*storemodels.ChannelSubscription); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*storemodels.ChannelSubscription)
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

// ListChannelSubscriptionsToRefresh provides a mock function with given fields:
func (_m *Store) ListChannelSubscriptionsToRefresh() ([]*storemodels.ChannelSubscription, error) {
	ret := _m.Called()

	var r0 []*storemodels.ChannelSubscription
	if rf, ok := ret.Get(0).(func() []*storemodels.ChannelSubscription); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*storemodels.ChannelSubscription)
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

// ListChatSubscriptionsToCheck provides a mock function with given fields:
func (_m *Store) ListChatSubscriptionsToCheck() ([]storemodels.ChatSubscription, error) {
	ret := _m.Called()

	var r0 []storemodels.ChatSubscription
	if rf, ok := ret.Get(0).(func() []storemodels.ChatSubscription); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]storemodels.ChatSubscription)
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

// ListGlobalSubscriptions provides a mock function with given fields:
func (_m *Store) ListGlobalSubscriptions() ([]*storemodels.GlobalSubscription, error) {
	ret := _m.Called()

	var r0 []*storemodels.GlobalSubscription
	if rf, ok := ret.Get(0).(func() []*storemodels.GlobalSubscription); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*storemodels.GlobalSubscription)
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

// ListGlobalSubscriptionsToRefresh provides a mock function with given fields:
func (_m *Store) ListGlobalSubscriptionsToRefresh() ([]*storemodels.GlobalSubscription, error) {
	ret := _m.Called()

	var r0 []*storemodels.GlobalSubscription
	if rf, ok := ret.Get(0).(func() []*storemodels.GlobalSubscription); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*storemodels.GlobalSubscription)
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

// LockPostByMMPostID provides a mock function with given fields: tx, messageID
func (_m *Store) LockPostByMMPostID(tx *sql.Tx, messageID string) error {
	ret := _m.Called(tx, messageID)

	var r0 error
	if rf, ok := ret.Get(0).(func(*sql.Tx, string) error); ok {
		r0 = rf(tx, messageID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// LockPostByMSTeamsPostID provides a mock function with given fields: tx, messageID
func (_m *Store) LockPostByMSTeamsPostID(tx *sql.Tx, messageID string) error {
	ret := _m.Called(tx, messageID)

	var r0 error
	if rf, ok := ret.Get(0).(func(*sql.Tx, string) error); ok {
		r0 = rf(tx, messageID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MattermostToTeamsUserID provides a mock function with given fields: userID
func (_m *Store) MattermostToTeamsUserID(userID string) (string, error) {
	ret := _m.Called(userID)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(userID)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(userID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PrefillWhitelist provides a mock function with given fields:
func (_m *Store) PrefillWhitelist() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RecoverPost provides a mock function with given fields: postID
func (_m *Store) RecoverPost(postID string) error {
	ret := _m.Called(postID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(postID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RollbackTx provides a mock function with given fields: tx
func (_m *Store) RollbackTx(tx *sql.Tx) error {
	ret := _m.Called(tx)

	var r0 error
	if rf, ok := ret.Get(0).(func(*sql.Tx) error); ok {
		r0 = rf(tx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveChannelSubscription provides a mock function with given fields: _a0, _a1
func (_m *Store) SaveChannelSubscription(_a0 *sql.Tx, _a1 storemodels.ChannelSubscription) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*sql.Tx, storemodels.ChannelSubscription) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveChatSubscription provides a mock function with given fields: _a0
func (_m *Store) SaveChatSubscription(_a0 storemodels.ChatSubscription) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(storemodels.ChatSubscription) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveGlobalSubscription provides a mock function with given fields: _a0
func (_m *Store) SaveGlobalSubscription(_a0 storemodels.GlobalSubscription) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(storemodels.GlobalSubscription) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetAvatarCache provides a mock function with given fields: userID, photo
func (_m *Store) SetAvatarCache(userID string, photo []byte) error {
	ret := _m.Called(userID, photo)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, []byte) error); ok {
		r0 = rf(userID, photo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetJobStatus provides a mock function with given fields: jobName, status
func (_m *Store) SetJobStatus(jobName string, status bool) error {
	ret := _m.Called(jobName, status)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, bool) error); ok {
		r0 = rf(jobName, status)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetPostLastUpdateAtByMSTeamsID provides a mock function with given fields: tx, postID, lastUpdateAt
func (_m *Store) SetPostLastUpdateAtByMSTeamsID(tx *sql.Tx, postID string, lastUpdateAt time.Time) error {
	ret := _m.Called(tx, postID, lastUpdateAt)

	var r0 error
	if rf, ok := ret.Get(0).(func(*sql.Tx, string, time.Time) error); ok {
		r0 = rf(tx, postID, lastUpdateAt)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetPostLastUpdateAtByMattermostID provides a mock function with given fields: tx, postID, lastUpdateAt
func (_m *Store) SetPostLastUpdateAtByMattermostID(tx *sql.Tx, postID string, lastUpdateAt time.Time) error {
	ret := _m.Called(tx, postID, lastUpdateAt)

	var r0 error
	if rf, ok := ret.Get(0).(func(*sql.Tx, string, time.Time) error); ok {
		r0 = rf(tx, postID, lastUpdateAt)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetUserInfo provides a mock function with given fields: userID, msTeamsUserID, token
func (_m *Store) SetUserInfo(userID string, msTeamsUserID string, token *oauth2.Token) error {
	ret := _m.Called(userID, msTeamsUserID, token)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, *oauth2.Token) error); ok {
		r0 = rf(userID, msTeamsUserID, token)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// StoreChannelLink provides a mock function with given fields: link
func (_m *Store) StoreChannelLink(link *storemodels.ChannelLink) error {
	ret := _m.Called(link)

	var r0 error
	if rf, ok := ret.Get(0).(func(*storemodels.ChannelLink) error); ok {
		r0 = rf(link)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// StoreDMAndGMChannelPromptTime provides a mock function with given fields: channelID, userID, timestamp
func (_m *Store) StoreDMAndGMChannelPromptTime(channelID string, userID string, timestamp time.Time) error {
	ret := _m.Called(channelID, userID, timestamp)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, time.Time) error); ok {
		r0 = rf(channelID, userID, timestamp)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// StoreOAuth2State provides a mock function with given fields: state
func (_m *Store) StoreOAuth2State(state string) error {
	ret := _m.Called(state)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(state)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// StoreUserInWhitelist provides a mock function with given fields: userID
func (_m *Store) StoreUserInWhitelist(userID string) error {
	ret := _m.Called(userID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(userID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// TeamsToMattermostUserID provides a mock function with given fields: userID
func (_m *Store) TeamsToMattermostUserID(userID string) (string, error) {
	ret := _m.Called(userID)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(userID)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(userID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateSubscriptionExpiresOn provides a mock function with given fields: subscriptionID, expiresOn
func (_m *Store) UpdateSubscriptionExpiresOn(subscriptionID string, expiresOn time.Time) error {
	ret := _m.Called(subscriptionID, expiresOn)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, time.Time) error); ok {
		r0 = rf(subscriptionID, expiresOn)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VerifyOAuth2State provides a mock function with given fields: state
func (_m *Store) VerifyOAuth2State(state string) error {
	ret := _m.Called(state)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(state)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewStore interface {
	mock.TestingT
	Cleanup(func())
}

// NewStore creates a new instance of Store. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewStore(t mockConstructorTestingTNewStore) *Store {
	mock := &Store{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
