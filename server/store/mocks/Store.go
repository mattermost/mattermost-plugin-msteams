// Code generated by mockery v2.18.0. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	oauth2 "golang.org/x/oauth2"

	storemodels "github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"

	time "time"
)

// Store is an autogenerated mock type for the Store type
type Store struct {
	mock.Mock
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

// DeleteUserFromWhitelist provides a mock function with given fields: userID
func (_m *Store) DeleteUserFromWhitelist(userID string) error {
	ret := _m.Called(userID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(userID)
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

// DeleteUserInvite provides a mock function with given fields: mmUserID
func (_m *Store) DeleteUserInvite(mmUserID string) error {
	ret := _m.Called(mmUserID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(mmUserID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
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

// GetConnectedUsersCount provides a mock function with given fields:
func (_m *Store) GetConnectedUsersCount() (int64, error) {
	ret := _m.Called()

	var r0 int64
	if rf, ok := ret.Get(0).(func() int64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int64)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
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

// GetHasConnectedCount provides a mock function with given fields:
func (_m *Store) GetHasConnectedCount() (int, error) {
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

// GetInvitedCount provides a mock function with given fields:
func (_m *Store) GetInvitedCount() (int, error) {
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

// GetInvitedUser provides a mock function with given fields: mmUserID
func (_m *Store) GetInvitedUser(mmUserID string) (*storemodels.InvitedUser, error) {
	ret := _m.Called(mmUserID)

	var r0 *storemodels.InvitedUser
	if rf, ok := ret.Get(0).(func(string) *storemodels.InvitedUser); ok {
		r0 = rf(mmUserID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*storemodels.InvitedUser)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(mmUserID)
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

// GetSubscriptionsLastActivityAt provides a mock function with given fields:
func (_m *Store) GetSubscriptionsLastActivityAt() (map[string]time.Time, error) {
	ret := _m.Called()

	var r0 map[string]time.Time
	if rf, ok := ret.Get(0).(func() map[string]time.Time); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]time.Time)
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

// GetUserConnectStatus provides a mock function with given fields: mmUserID
func (_m *Store) GetUserConnectStatus(mmUserID string) (*storemodels.UserConnectStatus, error) {
	ret := _m.Called(mmUserID)

	var r0 *storemodels.UserConnectStatus
	if rf, ok := ret.Get(0).(func(string) *storemodels.UserConnectStatus); ok {
		r0 = rf(mmUserID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*storemodels.UserConnectStatus)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(mmUserID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWhitelistCount provides a mock function with given fields:
func (_m *Store) GetWhitelistCount() (int, error) {
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

// GetWhitelistEmails provides a mock function with given fields: page, perPage
func (_m *Store) GetWhitelistEmails(page int, perPage int) ([]string, error) {
	ret := _m.Called(page, perPage)

	var r0 []string
	if rf, ok := ret.Get(0).(func(int, int) []string); ok {
		r0 = rf(page, perPage)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
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

// Init provides a mock function with given fields: remoteID
func (_m *Store) Init(remoteID string) error {
	ret := _m.Called(remoteID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(remoteID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// IsUserWhitelisted provides a mock function with given fields: userID
func (_m *Store) IsUserWhitelisted(userID string) (bool, error) {
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

// LinkPosts provides a mock function with given fields: postInfo
func (_m *Store) LinkPosts(postInfo storemodels.PostInfo) error {
	ret := _m.Called(postInfo)

	var r0 error
	if rf, ok := ret.Get(0).(func(storemodels.PostInfo) error); ok {
		r0 = rf(postInfo)
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

// ListChannelSubscriptionsToRefresh provides a mock function with given fields: certificate
func (_m *Store) ListChannelSubscriptionsToRefresh(certificate string) ([]*storemodels.ChannelSubscription, error) {
	ret := _m.Called(certificate)

	var r0 []*storemodels.ChannelSubscription
	if rf, ok := ret.Get(0).(func(string) []*storemodels.ChannelSubscription); ok {
		r0 = rf(certificate)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*storemodels.ChannelSubscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(certificate)
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

// ListGlobalSubscriptionsToRefresh provides a mock function with given fields: certificate
func (_m *Store) ListGlobalSubscriptionsToRefresh(certificate string) ([]*storemodels.GlobalSubscription, error) {
	ret := _m.Called(certificate)

	var r0 []*storemodels.GlobalSubscription
	if rf, ok := ret.Get(0).(func(string) []*storemodels.GlobalSubscription); ok {
		r0 = rf(certificate)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*storemodels.GlobalSubscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(certificate)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
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

// SaveChannelSubscription provides a mock function with given fields: subscription
func (_m *Store) SaveChannelSubscription(subscription storemodels.ChannelSubscription) error {
	ret := _m.Called(subscription)

	var r0 error
	if rf, ok := ret.Get(0).(func(storemodels.ChannelSubscription) error); ok {
		r0 = rf(subscription)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveChatSubscription provides a mock function with given fields: subscription
func (_m *Store) SaveChatSubscription(subscription storemodels.ChatSubscription) error {
	ret := _m.Called(subscription)

	var r0 error
	if rf, ok := ret.Get(0).(func(storemodels.ChatSubscription) error); ok {
		r0 = rf(subscription)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveGlobalSubscription provides a mock function with given fields: subscription
func (_m *Store) SaveGlobalSubscription(subscription storemodels.GlobalSubscription) error {
	ret := _m.Called(subscription)

	var r0 error
	if rf, ok := ret.Get(0).(func(storemodels.GlobalSubscription) error); ok {
		r0 = rf(subscription)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetPostLastUpdateAtByMSTeamsID provides a mock function with given fields: postID, lastUpdateAt
func (_m *Store) SetPostLastUpdateAtByMSTeamsID(postID string, lastUpdateAt time.Time) error {
	ret := _m.Called(postID, lastUpdateAt)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, time.Time) error); ok {
		r0 = rf(postID, lastUpdateAt)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetPostLastUpdateAtByMattermostID provides a mock function with given fields: postID, lastUpdateAt
func (_m *Store) SetPostLastUpdateAtByMattermostID(postID string, lastUpdateAt time.Time) error {
	ret := _m.Called(postID, lastUpdateAt)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, time.Time) error); ok {
		r0 = rf(postID, lastUpdateAt)
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

// SetWhitelist provides a mock function with given fields: userIDs, batchSize
func (_m *Store) SetWhitelist(userIDs []string, batchSize int) error {
	ret := _m.Called(userIDs, batchSize)

	var r0 error
	if rf, ok := ret.Get(0).(func([]string, int) error); ok {
		r0 = rf(userIDs, batchSize)
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

// StoreInvitedUser provides a mock function with given fields: invitedUser
func (_m *Store) StoreInvitedUser(invitedUser *storemodels.InvitedUser) error {
	ret := _m.Called(invitedUser)

	var r0 error
	if rf, ok := ret.Get(0).(func(*storemodels.InvitedUser) error); ok {
		r0 = rf(invitedUser)
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

// UpdateSubscriptionLastActivityAt provides a mock function with given fields: subscriptionID, lastActivityAt
func (_m *Store) UpdateSubscriptionLastActivityAt(subscriptionID string, lastActivityAt time.Time) error {
	ret := _m.Called(subscriptionID, lastActivityAt)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, time.Time) error); ok {
		r0 = rf(subscriptionID, lastActivityAt)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UserHasConnected provides a mock function with given fields: mmUserID
func (_m *Store) UserHasConnected(mmUserID string) (bool, error) {
	ret := _m.Called(mmUserID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string) bool); ok {
		r0 = rf(mmUserID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(mmUserID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
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
