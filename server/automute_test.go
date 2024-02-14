package main

import (
	"database/sql"
	"strings"
	"testing"

	storemocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type AutomuteAPIMock struct {
	*plugintest.API

	plugin *Plugin

	channels       map[string]*model.Channel
	preferences    map[string]model.Preference
	channelMembers map[string]*model.ChannelMember

	t *testing.T
}

func (a *AutomuteAPIMock) key(parts ...string) string {
	return strings.Join(parts, "-")
}

func (a *AutomuteAPIMock) GetPreferenceForUser(userID, category, name string) (model.Preference, *model.AppError) {
	a.t.Helper()

	preference, ok := a.preferences[a.key(userID, category, name)]
	if !ok {
		return model.Preference{}, &model.AppError{Message: "AutomuteAPIMock: Preference not found"}
	}
	return preference, nil
}

func (a *AutomuteAPIMock) UpdatePreferencesForUser(userID string, preferences []model.Preference) *model.AppError {
	a.t.Helper()

	for _, preference := range preferences {
		a.preferences[a.key(userID, preference.Category, preference.Name)] = preference
	}

	a.plugin.PreferencesHaveChanged(&plugin.Context{}, preferences)

	return nil
}

func (a *AutomuteAPIMock) CreateChannel(channel *model.Channel) (*model.Channel, *model.AppError) {
	a.t.Helper()

	a.channels[channel.Id] = channel

	a.plugin.ChannelHasBeenCreated(&plugin.Context{}, channel)

	return channel, nil
}

func (a *AutomuteAPIMock) GetDirectChannel(userID1, userID2 string) (*model.Channel, *model.AppError) {
	a.t.Helper()

	channel := &model.Channel{
		Id:   model.NewId(),
		Type: model.ChannelTypeDirect,
	}
	a.channels[channel.Id] = channel

	_, appErr := a.AddUserToChannel(channel.Id, userID1, "")
	require.Nil(a.t, appErr)
	_, appErr = a.AddUserToChannel(channel.Id, userID2, "")
	require.Nil(a.t, appErr)

	a.plugin.ChannelHasBeenCreated(&plugin.Context{}, channel)

	return channel, nil
}

func (a *AutomuteAPIMock) GetChannel(channelID string) (*model.Channel, *model.AppError) {
	a.t.Helper()

	channel, ok := a.channels[channelID]
	if !ok {
		return nil, &model.AppError{Message: "AutomuteAPIMock: Channel not found"}
	}

	return channel, nil
}

func (a *AutomuteAPIMock) AddUserToChannel(channelID, userID, asUserID string) (*model.ChannelMember, *model.AppError) {
	a.t.Helper()

	member := a.addMockChannelMember(channelID, userID)

	a.plugin.UserHasJoinedChannel(&plugin.Context{}, member, &model.User{Id: asUserID})

	return member, nil
}

func (a *AutomuteAPIMock) addMockChannelMember(channelID, userID string) *model.ChannelMember {
	member := &model.ChannelMember{
		UserId:      userID,
		ChannelId:   channelID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}

	a.channelMembers[a.key(channelID, userID)] = member

	return member
}

func (a *AutomuteAPIMock) GetChannelsForTeamForUser(teamID, userID string, includeDeleted bool) ([]*model.Channel, *model.AppError) {
	a.t.Helper()

	if teamID != "" || !includeDeleted {
		panic("Not implemented")
	}

	var channels []*model.Channel
	for _, channelMember := range a.channelMembers {
		if channelMember.UserId != userID {
			continue
		}

		channels = append(channels, a.channels[channelMember.ChannelId])
	}
	return channels, nil
}

func (a *AutomuteAPIMock) GetChannelMember(channelID, userID string) (*model.ChannelMember, *model.AppError) {
	a.t.Helper()

	member, ok := a.channelMembers[a.key(channelID, userID)]
	if !ok {
		return nil, &model.AppError{Message: "AutomuteAPIMock: Channel member not found"}
	}
	return member, nil
}

func (a *AutomuteAPIMock) GetChannelMembers(channelID string, page, perPage int) (model.ChannelMembers, *model.AppError) {
	a.t.Helper()

	var members []model.ChannelMember
	for _, member := range a.channelMembers {
		if member.ChannelId == channelID {
			members = append(members, *member)
		}
	}

	if page*perPage > len(members) {
		members = nil
	} else {
		members = members[page*perPage:]
	}

	if len(members) > perPage {
		members = members[:perPage]
	}

	return members, nil
}

func (a *AutomuteAPIMock) PatchChannelMembersNotifications(identifiers []*model.ChannelMemberIdentifier, notifyProps map[string]string) *model.AppError {
	a.t.Helper()

	for _, identifier := range identifiers {
		for propKey, propValue := range notifyProps {
			a.channelMembers[a.key(identifier.ChannelId, identifier.UserId)].NotifyProps[propKey] = propValue
		}
	}

	return nil
}

func (a *AutomuteAPIMock) LogDebug(msg string, keyValuePairs ...any) {
	a.t.Log(msg, keyValuePairs)
}

func (a *AutomuteAPIMock) LogInfo(msg string, keyValuePairs ...any) {
	a.t.Log(msg, keyValuePairs)
}

func (a *AutomuteAPIMock) LogError(msg string, keyValuePairs ...any) {
	a.t.Error(msg, keyValuePairs)
}

func (a *AutomuteAPIMock) LogWarn(msg string, keyValuePairs ...any) {
	a.t.Log(msg, keyValuePairs)
}

func newAutomuteTestPlugin(t *testing.T) *Plugin {
	mockAPI := &AutomuteAPIMock{
		API:            &plugintest.API{},
		channels:       make(map[string]*model.Channel),
		channelMembers: make(map[string]*model.ChannelMember),
		preferences:    make(map[string]model.Preference),
		t:              t,
	}
	t.Cleanup(func() { mockAPI.AssertExpectations(t) })

	p := newTestPlugin(t)
	p.SetAPI(mockAPI)

	mockAPI.plugin = p

	return p
}

func TestSetAutomuteEnabledForUser(t *testing.T) {
	p := newAutomuteTestPlugin(t)

	user := &model.User{Id: model.NewId()}

	channel, appErr := p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypeOpen})
	require.Nil(t, appErr)
	_, appErr = p.API.AddUserToChannel(channel.Id, user.Id, "")
	require.Nil(t, appErr)

	directChannel, appErr := p.API.GetDirectChannel(user.Id, model.NewId())
	require.Nil(t, appErr)

	mockLinkedChannel(p, channel)

	t.Run("initial conditions", func(t *testing.T) {
		assertChannelNotAutomuted(t, p, channel.Id, user.Id)
		assertChannelNotAutomuted(t, p, directChannel.Id, user.Id)
	})

	t.Run("should do nothing when false is passed and automuting has never been enabled", func(t *testing.T) {
		result, err := p.setAutomuteEnabledForUser(user.Id, false)

		assert.Equal(t, false, result)
		assert.NoError(t, err)

		assertChannelNotAutomuted(t, p, channel.Id, user.Id)
		assertChannelNotAutomuted(t, p, directChannel.Id, user.Id)
	})

	t.Run("should automute all channels when true is passed and automuting has never been enabled", func(t *testing.T) {
		result, err := p.setAutomuteEnabledForUser(user.Id, true)

		assert.Equal(t, true, result)
		assert.NoError(t, err)

		assertChannelAutomuted(t, p, channel.Id, user.Id)
		assertChannelAutomuted(t, p, directChannel.Id, user.Id)
	})

	t.Run("should do nothing when true is passed and automuting was last enabled", func(t *testing.T) {
		result, err := p.setAutomuteEnabledForUser(user.Id, true)

		assert.Equal(t, false, result)
		assert.NoError(t, err)

		assertChannelAutomuted(t, p, channel.Id, user.Id)
		assertChannelAutomuted(t, p, directChannel.Id, user.Id)
	})

	t.Run("should un-automute all channels when false is passed and automuting was last enabled", func(t *testing.T) {
		result, err := p.setAutomuteEnabledForUser(user.Id, false)

		assert.Equal(t, true, result)
		assert.NoError(t, err)

		assertChannelNotAutomuted(t, p, channel.Id, user.Id)
		assertChannelNotAutomuted(t, p, directChannel.Id, user.Id)
	})

	t.Run("should do nothing when false is passed and automuting was last disabled", func(t *testing.T) {
		result, err := p.setAutomuteEnabledForUser(user.Id, false)

		assert.Equal(t, false, result)
		assert.NoError(t, err)

		assertChannelNotAutomuted(t, p, channel.Id, user.Id)
		assertChannelNotAutomuted(t, p, directChannel.Id, user.Id)
	})

	t.Run("should automute all channels when true is passed and automuting was last disabled", func(t *testing.T) {
		result, err := p.setAutomuteEnabledForUser(user.Id, true)

		assert.Equal(t, true, result)
		assert.NoError(t, err)

		assertChannelAutomuted(t, p, channel.Id, user.Id)
		assertChannelAutomuted(t, p, directChannel.Id, user.Id)
	})
}

func TestChannelsAutomutedPreference(t *testing.T) {
	plugin := newAutomuteTestPlugin(t)

	user := &model.User{Id: model.NewId()}

	assert.False(t, plugin.getAutomuteIsEnabledForUser(user.Id))

	err := plugin.setAutomuteIsEnabledForUser(user.Id, true)
	require.Nil(t, err)

	assert.True(t, plugin.getAutomuteIsEnabledForUser(user.Id))

	err = plugin.setAutomuteIsEnabledForUser(user.Id, false)
	require.Nil(t, err)

	assert.False(t, plugin.getAutomuteIsEnabledForUser(user.Id))
}

func TestCanAutomuteChannel(t *testing.T) {
	t.Run("should return true for a linked channel", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		channel, appErr := p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypeOpen})
		require.Nil(t, appErr)
		mockLinkedChannel(p, channel)

		result, err := p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, true, result)

		channel, appErr = p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypePrivate})
		require.Nil(t, appErr)
		mockLinkedChannel(p, channel)

		result, err = p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("should return true for a DM/GM channel", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		channel, appErr := p.API.GetDirectChannel(model.NewId(), model.NewId())
		require.Nil(t, appErr)
		mockUnlinkedChannel(p, channel)

		result, err := p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, true, result)

		channel = &model.Channel{
			Id:   model.NewId(),
			Type: model.ChannelTypeGroup,
		}
		mockUnlinkedChannel(p, channel)

		result, err = p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("should return false for an unlinked channel", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		channel, appErr := p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypeOpen})
		require.Nil(t, appErr)
		mockUnlinkedChannel(p, channel)

		result, err := p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
	})
}

func assertUserHasAutomuteEnabled(t *testing.T, p *Plugin, userID string) {
	t.Helper()

	pref, appErr := p.API.GetPreferenceForUser(userID, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)

	assert.Nil(t, appErr)
	assert.Equal(t, "true", pref.Value)
}

func assertUserHasAutomuteDisabled(t *testing.T, p *Plugin, userID string) {
	t.Helper()

	pref, appErr := p.API.GetPreferenceForUser(userID, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
	if appErr == nil {
		assert.Equal(t, "false", pref.Value)
	}
}

func assertChannelAutomuted(t *testing.T, p *Plugin, channelID, userID string) {
	t.Helper()

	member, appErr := p.API.GetChannelMember(channelID, userID)
	require.Nil(t, appErr)

	assert.Equal(t, "true", member.NotifyProps[NotifyPropAutomuted])
	assert.Equal(t, model.ChannelMarkUnreadMention, member.NotifyProps[model.MarkUnreadNotifyProp])
}

func assertChannelNotAutomuted(t *testing.T, p *Plugin, channelID, userID string) {
	t.Helper()

	member, appErr := p.API.GetChannelMember(channelID, userID)
	require.Nil(t, appErr)

	if _, ok := member.NotifyProps[NotifyPropAutomuted]; ok {
		assert.Equal(t, "false", member.NotifyProps[NotifyPropAutomuted])
	}
	assert.Equal(t, model.ChannelMarkUnreadAll, member.NotifyProps[model.MarkUnreadNotifyProp])
}

func mockUserConnected(p *Plugin, userID string) {
	p.store.(*storemocks.Store).On("GetTokenForMattermostUser", userID).Return(&fakeToken, nil)
}

func mockUserNotConnected(p *Plugin, userID string) {
	p.store.(*storemocks.Store).On("GetTokenForMattermostUser", userID).Return(nil, sql.ErrNoRows)
}

// func setPrimaryPlatform(p *Plugin, userID string, value string) {
// 	_ = p.API.UpdatePreferencesForUser(userID, []model.Preference{{
// 		UserId:   userID,
// 		Category: PreferenceCategoryPlugin,
// 		Name:     PreferenceNamePlatform,
// 		Value:    value,
// 	}})
// }

func mockLinkedChannel(p *Plugin, channel *model.Channel) {
	link := &storemodels.ChannelLink{
		MattermostChannelID: channel.Id,
	}
	p.store.(*storemocks.Store).On("GetLinkByChannelID", channel.Id).Return(link, nil)
	p.store.(*storemocks.Store).On("CheckEnabledTeamByTeamID", channel.TeamId).Return(true)
}

func mockUnlinkedChannel(p *Plugin, channel *model.Channel) {
	p.store.(*storemocks.Store).On("GetLinkByChannelID", channel.Id).Return(nil, sql.ErrNoRows)
}
