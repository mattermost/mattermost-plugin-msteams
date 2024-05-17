// Code generated by mockery v2.18.0. DO NOT EDIT.

package mocks

import (
	metrics "github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	mock "github.com/stretchr/testify/mock"

	model "github.com/mattermost/mattermost/server/public/model"

	msteams "github.com/mattermost/mattermost-plugin-msteams/server/msteams"

	plugin "github.com/mattermost/mattermost/server/public/plugin"

	store "github.com/mattermost/mattermost-plugin-msteams/server/store"
)

// PluginIface is an autogenerated mock type for the PluginIface type
type PluginIface struct {
	mock.Mock
}

// ChannelShouldSync provides a mock function with given fields: channelID
func (_m *PluginIface) ChannelShouldSync(channelID string) (bool, error) {
	ret := _m.Called(channelID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string) bool); ok {
		r0 = rf(channelID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(channelID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ChannelShouldSyncCreated provides a mock function with given fields: channelID, senderID
func (_m *PluginIface) ChannelShouldSyncCreated(channelID string, senderID string) (bool, error) {
	ret := _m.Called(channelID, senderID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string, string) bool); ok {
		r0 = rf(channelID, senderID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(channelID, senderID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GenerateRandomPassword provides a mock function with given fields:
func (_m *PluginIface) GenerateRandomPassword() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetAPI provides a mock function with given fields:
func (_m *PluginIface) GetAPI() plugin.API {
	ret := _m.Called()

	var r0 plugin.API
	if rf, ok := ret.Get(0).(func() plugin.API); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(plugin.API)
		}
	}

	return r0
}

// GetBotUserID provides a mock function with given fields:
func (_m *PluginIface) GetBotUserID() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetBufferSizeForStreaming provides a mock function with given fields:
func (_m *PluginIface) GetBufferSizeForStreaming() int {
	ret := _m.Called()

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

// GetClientForApp provides a mock function with given fields:
func (_m *PluginIface) GetClientForApp() msteams.Client {
	ret := _m.Called()

	var r0 msteams.Client
	if rf, ok := ret.Get(0).(func() msteams.Client); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(msteams.Client)
		}
	}

	return r0
}

// GetClientForTeamsUser provides a mock function with given fields: _a0
func (_m *PluginIface) GetClientForTeamsUser(_a0 string) (msteams.Client, error) {
	ret := _m.Called(_a0)

	var r0 msteams.Client
	if rf, ok := ret.Get(0).(func(string) msteams.Client); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(msteams.Client)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetClientForUser provides a mock function with given fields: _a0
func (_m *PluginIface) GetClientForUser(_a0 string) (msteams.Client, error) {
	ret := _m.Called(_a0)

	var r0 msteams.Client
	if rf, ok := ret.Get(0).(func(string) msteams.Client); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(msteams.Client)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetMaxSizeForCompleteDownload provides a mock function with given fields:
func (_m *PluginIface) GetMaxSizeForCompleteDownload() int {
	ret := _m.Called()

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

// GetMetrics provides a mock function with given fields:
func (_m *PluginIface) GetMetrics() metrics.Metrics {
	ret := _m.Called()

	var r0 metrics.Metrics
	if rf, ok := ret.Get(0).(func() metrics.Metrics); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(metrics.Metrics)
		}
	}

	return r0
}

// GetRemoteID provides a mock function with given fields:
func (_m *PluginIface) GetRemoteID() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetSelectiveSync provides a mock function with given fields:
func (_m *PluginIface) GetSelectiveSync() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// GetStore provides a mock function with given fields:
func (_m *PluginIface) GetStore() store.Store {
	ret := _m.Called()

	var r0 store.Store
	if rf, ok := ret.Get(0).(func() store.Store); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.Store)
		}
	}

	return r0
}

// GetSyncDirectMessages provides a mock function with given fields:
func (_m *PluginIface) GetSyncDirectMessages() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// GetSyncFileAttachments provides a mock function with given fields:
func (_m *PluginIface) GetSyncFileAttachments() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// GetSyncGroupMessages provides a mock function with given fields:
func (_m *PluginIface) GetSyncGroupMessages() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// GetSyncGuestUsers provides a mock function with given fields:
func (_m *PluginIface) GetSyncGuestUsers() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// GetSyncLinkedChannels provides a mock function with given fields:
func (_m *PluginIface) GetSyncLinkedChannels() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// GetSyncReactions provides a mock function with given fields:
func (_m *PluginIface) GetSyncReactions() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// GetSyncRemoteOnly provides a mock function with given fields:
func (_m *PluginIface) GetSyncRemoteOnly() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// GetURL provides a mock function with given fields:
func (_m *PluginIface) GetURL() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// IsRemoteUser provides a mock function with given fields: user
func (_m *PluginIface) IsRemoteUser(user *model.User) bool {
	ret := _m.Called(user)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*model.User) bool); ok {
		r0 = rf(user)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// IsUserConnected provides a mock function with given fields: _a0
func (_m *PluginIface) IsUserConnected(_a0 string) (bool, error) {
	ret := _m.Called(_a0)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string) bool); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MessageFingerprint provides a mock function with given fields:
func (_m *PluginIface) MessageFingerprint() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

type mockConstructorTestingTNewPluginIface interface {
	mock.TestingT
	Cleanup(func())
}

// NewPluginIface creates a new instance of PluginIface. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewPluginIface(t mockConstructorTestingTNewPluginIface) *PluginIface {
	mock := &PluginIface{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
