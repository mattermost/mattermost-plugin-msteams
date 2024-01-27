package main

import (
	"database/sql"
	"strings"
	"testing"

	storemocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
)

type AutomuteAPIMock struct {
	*plugintest.API

	channels       map[string]*model.Channel
	preferences    map[string]model.Preference
	channelMembers map[string]*model.ChannelMember

	t *testing.T
}

func (a *AutomuteAPIMock) key(parts ...string) string {
	return strings.Join(parts, "-")
}

func (a *AutomuteAPIMock) GetPreferenceForUser(userID, category, name string) (model.Preference, *model.AppError) {
	preference, ok := a.preferences[a.key(userID, category, name)]
	if !ok {
		return model.Preference{}, &model.AppError{Message: "Preference not found"}
	}
	return preference, nil
}

func (a *AutomuteAPIMock) UpdatePreferencesForUser(userID string, preferences []model.Preference) *model.AppError {
	for _, preference := range preferences {
		a.preferences[a.key(userID, preference.Category, preference.Name)] = preference
	}

	return nil
}

func (a *AutomuteAPIMock) CreateChannel(channel *model.Channel) (*model.Channel, *model.AppError) {
	a.channels[channel.Id] = channel
	return channel, nil
}

func (a *AutomuteAPIMock) GetDirectChannel(userID1, userID2 string) (*model.Channel, *model.AppError) {
	channel, _ := a.CreateChannel(&model.Channel{
		Id:   model.NewId(),
		Type: model.ChannelTypeDirect,
	})
	a.AddUserToChannel(channel.Id, userID1, "")
	a.AddUserToChannel(channel.Id, userID2, "")
	return channel, nil
}

func (a *AutomuteAPIMock) AddUserToChannel(channelID, userID, asUserID string) (*model.ChannelMember, *model.AppError) {
	a.channelMembers[a.key(channelID, userID)] = &model.ChannelMember{
		UserId:      userID,
		ChannelId:   channelID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	return a.channelMembers[a.key(channelID, userID)], nil
}

func (a *AutomuteAPIMock) GetChannelsForTeamForUser(teamID, userID string, includeDeleted bool) ([]*model.Channel, *model.AppError) {
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
	member, ok := a.channelMembers[a.key(channelID, userID)]
	if !ok {
		return nil, &model.AppError{Message: "Channel member not found"}
	}
	return member, nil
}

func (a *AutomuteAPIMock) PatchChannelMembersNotifications(identifiers []*model.ChannelMemberIdentifier, notifyProps map[string]string) *model.AppError {
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

	return p
}

func TestShouldEnableAutomuteForUser(t *testing.T) {
	user := &model.User{Id: model.NewId()}

	t.Run("should return true when a user is both connected and has their primary platform set to Teams", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		mockUserConnected(p, user.Id)

		setPrimaryPlatform(p, user.Id, PreferenceValuePlatformMSTeams)

		result, err := p.shouldEnableAutomuteForUser(user.Id, false, false)

		assert.Equal(t, true, result)
		assert.NoError(t, err)
	})

	t.Run("should return false when a user is not connected but has their primary platform set to Teams", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		mockUserNotConnected(p, user.Id)

		setPrimaryPlatform(p, user.Id, PreferenceValuePlatformMSTeams)

		result, err := p.shouldEnableAutomuteForUser(user.Id, false, false)

		assert.Equal(t, false, result)
		assert.NoError(t, err)
	})

	t.Run("should return false when a user is connected but has their primary platform set to MM", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		mockUserConnected(p, user.Id)

		setPrimaryPlatform(p, user.Id, PreferenceValuePlatformMM)

		result, err := p.shouldEnableAutomuteForUser(user.Id, false, false)

		assert.Equal(t, false, result)
		assert.NoError(t, err)
	})

	t.Run("should return false when a user is not connected and has their primary platform set to MM", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		mockUserNotConnected(p, user.Id)

		setPrimaryPlatform(p, user.Id, PreferenceValuePlatformMM)

		result, err := p.shouldEnableAutomuteForUser(user.Id, false, false)

		assert.Equal(t, false, result)
		assert.NoError(t, err)
	})

	t.Run("should return true when a user is connected and we skip checking their primary platform", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		mockUserConnected(p, user.Id)

		// This would cause it to return false if we didn't skip the check
		setPrimaryPlatform(p, user.Id, PreferenceValuePlatformMM)

		result, err := p.shouldEnableAutomuteForUser(user.Id, false, true)

		assert.Equal(t, true, result)
		assert.NoError(t, err)
	})

	t.Run("should return false when a user is not connected and we're skipping checking their primary platform", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		mockUserNotConnected(p, user.Id)

		// This would cause it to return false if we didn't skip the check
		setPrimaryPlatform(p, user.Id, PreferenceValuePlatformMM)

		result, err := p.shouldEnableAutomuteForUser(user.Id, false, true)

		assert.Equal(t, false, result)
		assert.NoError(t, err)
	})

	t.Run("should return true when a user has Teams as their primary platform and we're skipping checking if they're connected", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		// This assumes that calling isUserConnected causes the mock to panic

		setPrimaryPlatform(p, user.Id, PreferenceValuePlatformMSTeams)

		result, err := p.shouldEnableAutomuteForUser(user.Id, true, false)

		assert.Equal(t, true, result)
		assert.NoError(t, err)
	})

	t.Run("should return true when a user has Teams as their primary platform and we're skipping checking if they're connected", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		// This assumes that calling isUserConnected causes the mock to panic

		setPrimaryPlatform(p, user.Id, PreferenceValuePlatformMM)

		result, err := p.shouldEnableAutomuteForUser(user.Id, true, false)

		assert.Equal(t, false, result)
		assert.NoError(t, err)
	})

	t.Run("should return true when we skip both checks", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		// This assumes that calling isUserConnected causes the mock to panic

		// This would cause it to return false if we didn't skip the check
		setPrimaryPlatform(p, user.Id, PreferenceValuePlatformMM)

		result, err := p.shouldEnableAutomuteForUser(user.Id, true, false)

		assert.Equal(t, false, result)
		assert.NoError(t, err)
	})
}

func TestSetAutomuteEnabledForUser(t *testing.T) {
	p := newAutomuteTestPlugin(t)

	user := &model.User{Id: model.NewId()}

	channel, _ := p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypeOpen})
	p.API.AddUserToChannel(channel.Id, user.Id, "")
	directChannel, _ := p.API.GetDirectChannel(user.Id, model.NewId())

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

	plugin.setAutomuteIsEnabledForUser(user.Id, true)

	assert.True(t, plugin.getAutomuteIsEnabledForUser(user.Id))

	plugin.setAutomuteIsEnabledForUser(user.Id, false)

	assert.False(t, plugin.getAutomuteIsEnabledForUser(user.Id))
}

func TestCanAutomuteChannel(t *testing.T) {
	t.Run("should return true for a linked channel", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		channel, _ := p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypeOpen})
		mockLinkedChannel(p, channel)

		result, err := p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, true, result)

		channel, _ = p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypePrivate})
		mockLinkedChannel(p, channel)

		result, err = p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("should return true for a DM/GM channel", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		channel, _ := p.API.GetDirectChannel(model.NewId(), model.NewId())
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

		channel, _ := p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypeOpen})
		mockUnlinkedChannel(p, channel)

		result, err := p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
	})
}

func assertChannelAutomuted(t *testing.T, p *Plugin, channelID, userID string) {
	t.Helper()

	member, _ := p.API.GetChannelMember(channelID, userID)

	assert.Equal(t, "true", member.NotifyProps[NotifyPropAutomuted])
	assert.Equal(t, model.ChannelMarkUnreadMention, member.NotifyProps[model.MarkUnreadNotifyProp])
}

func assertChannelNotAutomuted(t *testing.T, p *Plugin, channelID, userID string) {
	t.Helper()

	member, _ := p.API.GetChannelMember(channelID, userID)

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

func setPrimaryPlatform(p *Plugin, userID string, value string) {
	p.API.UpdatePreferencesForUser(userID, []model.Preference{{
		UserId:   userID,
		Category: PreferenceCategoryPlugin,
		Name:     PreferenceNamePlatform,
		Value:    value,
	}})
}

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
