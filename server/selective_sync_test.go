package main

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatShouldSync(t *testing.T) {
	th := setupTestHelper(t)
	th.setPluginConfiguration(t, func(c *configuration) {
		c.SelectiveSync = true
		c.SyncChats = true
	})

	t.Run("direct message, sync chats disabled", func(t *testing.T) {
		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.SyncChats = false
		})

		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupUser(t, team)
		channel, err := th.p.apiClient.Channel.GetDirect(user1.Id, user2.Id)
		require.NoError(t, err)

		chatShouldSync, containsRemoteUser, members, discardReason, err := th.p.ChatShouldSync(channel)
		require.NoError(t, err)
		assert.False(t, chatShouldSync)
		assert.False(t, containsRemoteUser) // False because chatShouldSync is false
		assert.Len(t, members, 0)
		assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
	})

	t.Run("group message, sync chats disabled", func(t *testing.T) {
		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.SyncChats = false
		})

		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupUser(t, team)
		channel, err := th.p.apiClient.Channel.GetGroup([]string{user1.Id, user2.Id, user3.Id})
		require.NoError(t, err)

		chatShouldSync, containsRemoteUser, members, discardReason, err := th.p.ChatShouldSync(channel)
		require.NoError(t, err)
		assert.False(t, chatShouldSync)
		assert.False(t, containsRemoteUser) // False because chatShouldSync is false
		assert.Len(t, members, 0)
		assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
	})

	t.Run("selective sync disabled", func(t *testing.T) {
		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.SelectiveSync = false
		})

		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupUser(t, team)
		channel, err := th.p.apiClient.Channel.GetDirect(user1.Id, user2.Id)
		require.NoError(t, err)

		chatShouldSync, containsRemoteUser, members, discardReason, err := th.p.ChatShouldSync(channel)
		require.NoError(t, err)
		assert.True(t, chatShouldSync)
		assert.True(t, containsRemoteUser)
		assert.Len(t, members, 2)
		assert.Equal(t, metrics.DiscardedReasonNone, discardReason)
	})

	t.Run("not a direct message or group message", func(t *testing.T) {
		team := th.SetupTeam(t)
		channel := th.SetupPublicChannel(t, team)

		chatShouldSync, containsRemoteUser, members, discardReason, err := th.p.ChatShouldSync(channel)
		require.NoError(t, err)
		assert.False(t, chatShouldSync)
		assert.False(t, containsRemoteUser)
		assert.Empty(t, members)
		assert.Equal(t, metrics.DiscardedReasonOther, discardReason)
	})

	t.Run("dm with self", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		channel, err := th.p.apiClient.Channel.GetDirect(user1.Id, user1.Id)
		require.NoError(t, err)

		chatShouldSync, containsRemoteUser, members, discardReason, err := th.p.ChatShouldSync(channel)
		require.NoError(t, err)
		assert.False(t, chatShouldSync)
		assert.False(t, containsRemoteUser)
		assert.Len(t, members, 1)
		assert.Equal(t, metrics.DiscardedReasonSelectiveSync, discardReason)
	})

	t.Run("dm between two local users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)
		channel, err := th.p.apiClient.Channel.GetDirect(user1.Id, user2.Id)
		require.NoError(t, err)

		chatShouldSync, containsRemoteUser, members, discardReason, err := th.p.ChatShouldSync(channel)
		require.NoError(t, err)
		assert.False(t, chatShouldSync)
		assert.False(t, containsRemoteUser)
		assert.Len(t, members, 2)
		assert.Equal(t, metrics.DiscardedReasonSelectiveSync, discardReason)
	})

	t.Run("dm between two remote users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupRemoteUser(t, team)
		channel, err := th.p.apiClient.Channel.GetDirect(user1.Id, user2.Id)
		require.NoError(t, err)

		chatShouldSync, containsRemoteUser, members, discardReason, err := th.p.ChatShouldSync(channel)
		require.NoError(t, err)
		assert.False(t, chatShouldSync)
		assert.True(t, containsRemoteUser)
		assert.Len(t, members, 2)
		assert.Equal(t, metrics.DiscardedReasonSelectiveSync, discardReason)
	})

	t.Run("dm between a local and a remote user", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupUser(t, team)
		channel, err := th.p.apiClient.Channel.GetDirect(user1.Id, user2.Id)
		require.NoError(t, err)

		chatShouldSync, containsRemoteUser, members, discardReason, err := th.p.ChatShouldSync(channel)
		require.NoError(t, err)
		assert.True(t, chatShouldSync)
		assert.True(t, containsRemoteUser)
		assert.Len(t, members, 2)
		assert.Equal(t, metrics.DiscardedReasonNone, discardReason)
	})

	t.Run("gm between three local users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupUser(t, team)
		channel, err := th.p.apiClient.Channel.GetGroup([]string{user1.Id, user2.Id, user3.Id})
		require.NoError(t, err)

		chatShouldSync, containsRemoteUser, members, discardReason, err := th.p.ChatShouldSync(channel)
		require.NoError(t, err)
		assert.False(t, chatShouldSync)
		assert.False(t, containsRemoteUser)
		assert.Len(t, members, 3)
		assert.Equal(t, metrics.DiscardedReasonSelectiveSync, discardReason)
	})

	t.Run("gm between three remote users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupRemoteUser(t, team)
		user3 := th.SetupRemoteUser(t, team)
		channel, err := th.p.apiClient.Channel.GetGroup([]string{user1.Id, user2.Id, user3.Id})
		require.NoError(t, err)

		chatShouldSync, containsRemoteUser, members, discardReason, err := th.p.ChatShouldSync(channel)
		require.NoError(t, err)
		assert.False(t, chatShouldSync)
		assert.True(t, containsRemoteUser)
		assert.Len(t, members, 3)
		assert.Equal(t, metrics.DiscardedReasonSelectiveSync, discardReason)
	})

	t.Run("gm between one local user and two remote users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupRemoteUser(t, team)
		channel, err := th.p.apiClient.Channel.GetGroup([]string{user1.Id, user2.Id, user3.Id})
		require.NoError(t, err)

		chatShouldSync, containsRemoteUser, members, discardReason, err := th.p.ChatShouldSync(channel)
		require.NoError(t, err)
		assert.True(t, chatShouldSync)
		assert.True(t, containsRemoteUser)
		assert.Len(t, members, 3)
		assert.Equal(t, metrics.DiscardedReasonNone, discardReason)
	})

	t.Run("gm between two local users and one remote users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupUser(t, team)
		channel, err := th.p.apiClient.Channel.GetGroup([]string{user1.Id, user2.Id, user3.Id})
		require.NoError(t, err)

		chatShouldSync, containsRemoteUser, members, discardReason, err := th.p.ChatShouldSync(channel)
		require.NoError(t, err)
		assert.False(t, chatShouldSync)
		assert.True(t, containsRemoteUser)
		assert.Len(t, members, 3)
		assert.Equal(t, metrics.DiscardedReasonSelectiveSync, discardReason)
	})
}
