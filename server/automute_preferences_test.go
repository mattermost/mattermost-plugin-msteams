package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/stretchr/testify/assert"
)

func TestUpdateAutomutingOnPreferencesChanged(t *testing.T) {
	setup := func(t *testing.T) (*Plugin, *model.User, *model.Channel, *model.Channel, *model.Channel) {
		t.Helper()

		p := newAutomuteTestPlugin(t)

		user := &model.User{Id: model.NewId()}
		mockUserConnected(p, user.Id)

		linkedChannel, _ := p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypeOpen})
		p.API.AddUserToChannel(linkedChannel.Id, user.Id, "")
		mockLinkedChannel(p, linkedChannel)

		unlinkedChannel, _ := p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypeOpen})
		p.API.AddUserToChannel(unlinkedChannel.Id, user.Id, "")
		mockUnlinkedChannel(p, unlinkedChannel)

		dmChannel, _ := p.API.GetDirectChannel(user.Id, model.NewId())

		assertChannelNotAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, dmChannel.Id, user.Id)

		return p, user, linkedChannel, unlinkedChannel, dmChannel
	}

	t.Run("should mute linked channels when their primary platform changes from MM to MS Teams", func(t *testing.T) {
		p, user, linkedChannel, unlinkedChannel, dmChannel := setup(t)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMM,
			},
		})

		pref, _ := p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "", pref.Value)

		assertChannelNotAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, dmChannel.Id, user.Id)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		pref, _ = p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "true", pref.Value)

		assertChannelAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)
		assertChannelAutomuted(t, p, dmChannel.Id, user.Id)
	})

	t.Run("should unmute linked channels when their primary platform changes from MS Teams to MM", func(t *testing.T) {
		p, user, linkedChannel, unlinkedChannel, dmChannel := setup(t)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		pref, _ := p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "true", pref.Value)

		assertChannelAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)
		assertChannelAutomuted(t, p, dmChannel.Id, user.Id)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMM,
			},
		})

		pref, _ = p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "false", pref.Value)

		assertChannelNotAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, dmChannel.Id, user.Id)
	})

	t.Run("should do nothing when unrelated preferences change", func(t *testing.T) {
		p, user, _, _, _ := setup(t)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: model.PreferenceCategoryDisplaySettings,
				Name:     model.PreferenceNameChannelDisplayMode,
				Value:    "full",
			},
		})
	})

	t.Run("should do nothing when an unconnected user turns on automuting", func(t *testing.T) {
		p, _, linkedChannel, unlinkedChannel, _ := setup(t)

		unconnectedUser := &model.User{Id: model.NewId()}
		mockUserNotConnected(p, unconnectedUser.Id)

		p.API.AddUserToChannel(linkedChannel.Id, unconnectedUser.Id, "")
		p.API.AddUserToChannel(unlinkedChannel.Id, unconnectedUser.Id, "")

		dmChannel, _ := p.API.GetDirectChannel(unconnectedUser.Id, model.NewId())

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   unconnectedUser.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		pref, _ := p.API.GetPreferenceForUser(unconnectedUser.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "", pref.Value)

		assertChannelNotAutomuted(t, p, linkedChannel.Id, unconnectedUser.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, unconnectedUser.Id)
		assertChannelNotAutomuted(t, p, dmChannel.Id, unconnectedUser.Id)
	})

	t.Run("should not affect other users when a connected user turns on automuting", func(t *testing.T) {
		p, user, linkedChannel, unlinkedChannel, _ := setup(t)

		connectedUser := &model.User{Id: model.NewId()}
		mockUserConnected(p, connectedUser.Id)

		p.API.AddUserToChannel(linkedChannel.Id, connectedUser.Id, "")
		p.API.AddUserToChannel(unlinkedChannel.Id, connectedUser.Id, "")

		unconnectedUser := &model.User{Id: model.NewId()}
		mockUserNotConnected(p, unconnectedUser.Id)

		p.API.AddUserToChannel(linkedChannel.Id, unconnectedUser.Id, "")
		p.API.AddUserToChannel(unlinkedChannel.Id, unconnectedUser.Id, "")

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		pref, _ := p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "true", pref.Value)

		assertChannelAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)

		pref, _ = p.API.GetPreferenceForUser(connectedUser.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "", pref.Value)

		assertChannelNotAutomuted(t, p, linkedChannel.Id, connectedUser.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, connectedUser.Id)

		pref, _ = p.API.GetPreferenceForUser(unconnectedUser.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "", pref.Value)

		assertChannelNotAutomuted(t, p, linkedChannel.Id, unconnectedUser.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, unconnectedUser.Id)
	})

	t.Run("should be able to mute a lot of channels at once", func(t *testing.T) {
		p, user, _, _, _ := setup(t)

		numChannels := 1000
		channels := make([]*model.Channel, numChannels)
		for i := 0; i < numChannels; i++ {
			channel, _ := p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypeOpen})
			p.API.AddUserToChannel(channel.Id, user.Id, "")
			mockLinkedChannel(p, channel)

			channels[i] = channel
		}

		for _, channel := range channels {
			assertChannelNotAutomuted(t, p, channel.Id, user.Id)
		}

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		pref, _ := p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "true", pref.Value)

		for _, channel := range channels {
			assertChannelAutomuted(t, p, channel.Id, user.Id)
		}

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMM,
			},
		})

		pref, _ = p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "false", pref.Value)

		for _, channel := range channels {
			assertChannelNotAutomuted(t, p, channel.Id, user.Id)
		}
	})
}

func TestGetUsersWhoChangedPlatform(t *testing.T) {
	preferences := []model.Preference{
		{
			UserId:   "user1",
			Category: PreferenceCategoryPlugin,
			Name:     PreferenceNamePlatform,
			Value:    PreferenceValuePlatformMM,
		},
		{
			UserId:   "user2",
			Category: PreferenceCategoryPlugin,
			Name:     PreferenceNamePlatform,
			Value:    PreferenceValuePlatformMSTeams,
		},
		{
			UserId:   "user3",
			Category: model.PreferenceCategoryDisplaySettings,
			Name:     model.PreferenceNameChannelDisplayMode,
			Value:    "full",
		},
	}

	usersWithTeamsPrimary, usersWithMMPrimary := getUsersWhoChangedPlatform(preferences)

	assert.Equal(t, []string{"user2"}, usersWithTeamsPrimary)
	assert.Equal(t, []string{"user1"}, usersWithMMPrimary)
}
