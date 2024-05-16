package main

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetAutomuteEnabledForUser(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	user := th.SetupUser(t, team)

	channel := th.SetupPublicChannel(t, team, WithMembers(user))

	otherUser := th.SetupUser(t, team)
	directChannel, appErr := th.p.API.GetDirectChannel(user.Id, otherUser.Id)
	require.Nil(t, appErr)

	th.LinkChannel(t, team, channel, user)

	t.Run("initial conditions", func(t *testing.T) {
		assertChannelNotAutomuted(t, th.p, channel.Id, user.Id)
		assertChannelNotAutomuted(t, th.p, directChannel.Id, user.Id)
	})

	t.Run("should do nothing when false is passed and automuting has never been enabled", func(t *testing.T) {
		result, err := th.p.setAutomuteEnabledForUser(user.Id, false)

		assert.Equal(t, false, result)
		assert.NoError(t, err)

		assertChannelNotAutomuted(t, th.p, channel.Id, user.Id)
		assertChannelNotAutomuted(t, th.p, directChannel.Id, user.Id)
	})

	t.Run("should automute all channels when true is passed and automuting has never been enabled", func(t *testing.T) {
		result, err := th.p.setAutomuteEnabledForUser(user.Id, true)

		assert.Equal(t, true, result)
		assert.NoError(t, err)

		assertChannelAutomuted(t, th.p, channel.Id, user.Id)
		assertChannelAutomuted(t, th.p, directChannel.Id, user.Id)
	})

	t.Run("should do nothing when true is passed and automuting was last enabled", func(t *testing.T) {
		result, err := th.p.setAutomuteEnabledForUser(user.Id, true)

		assert.Equal(t, false, result)
		assert.NoError(t, err)

		assertChannelAutomuted(t, th.p, channel.Id, user.Id)
		assertChannelAutomuted(t, th.p, directChannel.Id, user.Id)
	})

	t.Run("should un-automute all channels when false is passed and automuting was last enabled", func(t *testing.T) {
		result, err := th.p.setAutomuteEnabledForUser(user.Id, false)

		assert.Equal(t, true, result)
		assert.NoError(t, err)

		assertChannelNotAutomuted(t, th.p, channel.Id, user.Id)
		assertChannelNotAutomuted(t, th.p, directChannel.Id, user.Id)
	})

	t.Run("should do nothing when false is passed and automuting was last disabled", func(t *testing.T) {
		result, err := th.p.setAutomuteEnabledForUser(user.Id, false)

		assert.Equal(t, false, result)
		assert.NoError(t, err)

		assertChannelNotAutomuted(t, th.p, channel.Id, user.Id)
		assertChannelNotAutomuted(t, th.p, directChannel.Id, user.Id)
	})

	t.Run("should automute all channels when true is passed and automuting was last disabled", func(t *testing.T) {
		result, err := th.p.setAutomuteEnabledForUser(user.Id, true)

		assert.Equal(t, true, result)
		assert.NoError(t, err)

		assertChannelAutomuted(t, th.p, channel.Id, user.Id)
		assertChannelAutomuted(t, th.p, directChannel.Id, user.Id)
	})
}

func TestChannelsAutomutedPreference(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user := th.SetupUser(t, team)

	assert.False(t, th.p.getAutomuteIsEnabledForUser(user.Id))

	err := th.p.setAutomuteIsEnabledForUser(user.Id, true)
	require.Nil(t, err)

	assert.True(t, th.p.getAutomuteIsEnabledForUser(user.Id))

	err = th.p.setAutomuteIsEnabledForUser(user.Id, false)
	require.Nil(t, err)

	assert.False(t, th.p.getAutomuteIsEnabledForUser(user.Id))
}

func TestCanAutomuteChannel(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)
	user := th.SetupUser(t, team)

	t.Run("should return true for a linked channel", func(t *testing.T) {
		channel := th.SetupPublicChannel(t, team)
		th.LinkChannel(t, team, channel, user)

		result, err := th.p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, true, result)

		channel = th.SetupPrivateChannel(t, team)
		th.LinkChannel(t, team, channel, user)

		result, err = th.p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("should return true for a DM/GM channel with normal user", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)

		channel, appErr := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, appErr)

		result, err := th.p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, true, result)

		channel = &model.Channel{
			Id:   model.NewId(),
			Type: model.ChannelTypeGroup,
		}

		result, err = th.p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("should return false for a DM/GM channel with guest user", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		user2 := th.SetupGuestUser(t, team)

		channel, appErr := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, appErr)

		result, err := th.p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
	})

	t.Run("should return false for a DM/GM channel with bot user", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		bot := th.CreateBot(t)

		channel, appErr := th.p.API.GetDirectChannel(user1.Id, bot.UserId)
		require.Nil(t, appErr)

		result, err := th.p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
	})

	t.Run("should return false for an unlinked channel", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team)

		result, err := th.p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
	})

	t.Run("should return false for a DM/GM channel with guest user", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		user2 := th.SetupGuestUser(t, team)

		channel, appErr := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, appErr)

		result, err := th.p.canAutomuteChannel(channel)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
	})

	t.Run("should return false for a DM/GM channel with bot user", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		bot := th.CreateBot(t)

		channel, appErr := th.p.API.GetDirectChannel(user1.Id, bot.UserId)
		require.Nil(t, appErr)

		result, err := th.p.canAutomuteChannel(channel)
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

	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		member, appErr := p.API.GetChannelMember(channelID, userID)
		require.Nil(t, appErr)

		assert.Equal(t, "true", member.NotifyProps[NotifyPropAutomuted])
		assert.Equal(t, model.ChannelMarkUnreadMention, member.NotifyProps[model.MarkUnreadNotifyProp])
	}, 1*time.Second, 10*time.Millisecond)
}

func assertChannelNotAutomuted(t *testing.T, p *Plugin, channelID, userID string) {
	t.Helper()

	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		member, appErr := p.API.GetChannelMember(channelID, userID)
		require.Nil(t, appErr)

		if _, ok := member.NotifyProps[NotifyPropAutomuted]; ok {
			assert.Equal(t, "false", member.NotifyProps[NotifyPropAutomuted])
		}
		assert.Equal(t, model.ChannelMarkUnreadAll, member.NotifyProps[model.MarkUnreadNotifyProp])
	}, 1*time.Second, 10*time.Millisecond)
}

func assertChannelManuallyMuted(t *testing.T, p *Plugin, channelID, userID string) {
	t.Helper()

	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		member, appErr := p.API.GetChannelMember(channelID, userID)
		require.Nil(t, appErr)

		assert.Equal(t, "", member.NotifyProps[NotifyPropAutomuted])
		assert.Equal(t, model.ChannelMarkUnreadMention, member.NotifyProps[model.MarkUnreadNotifyProp])
	}, 1*time.Second, 10*time.Millisecond)
}
