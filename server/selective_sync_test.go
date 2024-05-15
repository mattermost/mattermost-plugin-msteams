package main

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatMembersSpanPlatforms(t *testing.T) {
	th := setupTestHelper(t)

	t.Run("empty set of channel members", func(t *testing.T) {
		chatMembersSpanPlatforms, appErr := th.p.ChatMembersSpanPlatforms([]*model.ChannelMember{})
		require.Nil(t, appErr)
		require.False(t, chatMembersSpanPlatforms)
	})

	t.Run("single user", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)

		chatShouldSync, appErr := th.p.ChatMembersSpanPlatforms([]*model.ChannelMember{
			{UserId: user1.Id},
		})

		require.Error(t, appErr)
		assert.False(t, chatShouldSync)
	})

	t.Run("user with empty id", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		chatMembersSpanPlatforms, appErr := th.p.ChatMembersSpanPlatforms([]*model.ChannelMember{{UserId: user1.Id}, {UserId: ""}})
		require.Error(t, appErr)
		require.False(t, chatMembersSpanPlatforms)
	})

	t.Run("dm between two local users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)

		chatShouldSync, appErr := th.p.ChatMembersSpanPlatforms([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
		})
		require.Nil(t, appErr)
		assert.False(t, chatShouldSync)
	})

	t.Run("dm between two remote users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupRemoteUser(t, team)

		var appErr *model.AppError

		user1.RemoteId = model.NewString(th.p.remoteID)
		user1, appErr = th.p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user2.RemoteId = model.NewString(th.p.remoteID)
		user2, appErr = th.p.API.UpdateUser(user2)
		require.Nil(t, appErr)

		chatShouldSync, err := th.p.ChatMembersSpanPlatforms([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
		})
		require.NoError(t, err)
		assert.False(t, chatShouldSync)
	})

	t.Run("dm between a local and a remote user", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupUser(t, team)

		var appErr *model.AppError

		user1.RemoteId = model.NewString(th.p.remoteID)
		user1, appErr = th.p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		chatShouldSync, err := th.p.ChatMembersSpanPlatforms([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
		})
		require.NoError(t, err)
		assert.True(t, chatShouldSync)
	})

	t.Run("dm between a local and a local user with teams as primary platform", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)

		err := th.p.setPrimaryPlatform(user2.Id, storemodels.PreferenceValuePlatformMSTeams)
		require.NoError(t, err)

		_, appErr := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, appErr)

		chatShouldSync, err := th.p.ChatMembersSpanPlatforms([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
		})
		require.Nil(t, err)
		assert.True(t, chatShouldSync)
	})

	t.Run("gm between three local users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupUser(t, team)

		chatShouldSync, err := th.p.ChatMembersSpanPlatforms([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
			{UserId: user3.Id},
		})
		require.Nil(t, err)
		assert.False(t, chatShouldSync)
	})

	t.Run("gm between three remote users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupRemoteUser(t, team)
		user3 := th.SetupRemoteUser(t, team)

		var appErr *model.AppError

		user1.RemoteId = model.NewString(th.p.remoteID)
		user1, appErr = th.p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user2.RemoteId = model.NewString(th.p.remoteID)
		user2, appErr = th.p.API.UpdateUser(user2)
		require.Nil(t, appErr)

		user3.RemoteId = model.NewString(th.p.remoteID)
		user3, appErr = th.p.API.UpdateUser(user3)
		require.Nil(t, appErr)

		chatShouldSync, err := th.p.ChatMembersSpanPlatforms([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
			{UserId: user3.Id},
		})
		require.NoError(t, err)
		assert.False(t, chatShouldSync)
	})

	t.Run("gm between a mixture of local and remote users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupRemoteUser(t, team)

		var appErr *model.AppError

		user1.RemoteId = model.NewString(th.p.remoteID)
		user1, appErr = th.p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user3.RemoteId = model.NewString(th.p.remoteID)
		user3, appErr = th.p.API.UpdateUser(user3)
		require.Nil(t, appErr)

		chatShouldSync, err := th.p.ChatMembersSpanPlatforms([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
			{UserId: user3.Id},
		})
		require.NoError(t, err)
		assert.True(t, chatShouldSync)
	})

	t.Run("gm between two local users and a local user with teams as primary platform", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupUser(t, team)

		err := th.p.setPrimaryPlatform(user3.Id, storemodels.PreferenceValuePlatformMSTeams)
		require.NoError(t, err)

		_, appErr := th.p.API.GetGroupChannel([]string{user1.Id, user2.Id, user3.Id})
		require.Nil(t, appErr)

		chatShouldSync, err := th.p.ChatMembersSpanPlatforms([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
			{UserId: user3.Id},
		})
		require.NoError(t, err)
		assert.True(t, chatShouldSync)
	})
}

func TestMembersContainRemote(t *testing.T) {
	th := setupTestHelper(t)

	t.Run("empty set of channel members", func(t *testing.T) {
		chatMembersSpanPlatforms, err := th.p.MembersContainsRemote([]*model.ChannelMember{})
		require.NoError(t, err)
		require.False(t, chatMembersSpanPlatforms)
	})

	t.Run("single user", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)

		remoteUsers, err := th.p.MembersContainsRemote([]*model.ChannelMember{
			{UserId: user1.Id},
		})

		require.NoError(t, err)
		assert.True(t, remoteUsers)
	})

	t.Run("user with empty id", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		chatMembersSpanPlatforms, err := th.p.MembersContainsRemote([]*model.ChannelMember{
			{UserId: ""},
			{UserId: user1.Id}})
		require.Error(t, err)
		require.False(t, chatMembersSpanPlatforms)
	})

	t.Run("dm between two local users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)

		remoteUsers, err := th.p.MembersContainsRemote([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
		})
		require.NoError(t, err)
		assert.False(t, remoteUsers)
	})

	t.Run("dm between two remote users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupRemoteUser(t, team)

		var appErr *model.AppError

		user1.RemoteId = model.NewString(th.p.remoteID)
		user1, appErr = th.p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user2.RemoteId = model.NewString(th.p.remoteID)
		user2, appErr = th.p.API.UpdateUser(user2)
		require.Nil(t, appErr)

		remoteUsers, err := th.p.MembersContainsRemote([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
		})
		require.NoError(t, err)
		assert.True(t, remoteUsers)
	})

	t.Run("dm between a local and a remote user", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupUser(t, team)

		remoteUsers, err := th.p.MembersContainsRemote([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
		})
		require.NoError(t, err)
		assert.True(t, remoteUsers)
	})

	t.Run("dm between a local and a local user with teams as primary platform", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)

		err := th.p.setPrimaryPlatform(user2.Id, storemodels.PreferenceValuePlatformMSTeams)
		require.NoError(t, err)

		_, appErr := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, appErr)

		remoteUsers, err := th.p.MembersContainsRemote([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
		})
		require.NoError(t, err)
		assert.False(t, remoteUsers)
	})

	t.Run("gm between three local users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupUser(t, team)

		remoteUsers, err := th.p.MembersContainsRemote([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
			{UserId: user3.Id},
		})
		require.NoError(t, err)
		assert.False(t, remoteUsers)
	})

	t.Run("gm between three remote users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupRemoteUser(t, team)
		user3 := th.SetupRemoteUser(t, team)

		var appErr *model.AppError

		user1.RemoteId = model.NewString(th.p.remoteID)
		user1, appErr = th.p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user2.RemoteId = model.NewString(th.p.remoteID)
		user2, appErr = th.p.API.UpdateUser(user2)
		require.Nil(t, appErr)

		user3.RemoteId = model.NewString(th.p.remoteID)
		user3, appErr = th.p.API.UpdateUser(user3)
		require.Nil(t, appErr)

		remoteUsers, err := th.p.MembersContainsRemote([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
			{UserId: user3.Id},
		})
		require.NoError(t, err)
		assert.True(t, remoteUsers)
	})

	t.Run("gm between a mixture of local and remote users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupRemoteUser(t, team)

		var appErr *model.AppError

		user1.RemoteId = model.NewString(th.p.remoteID)
		user1, appErr = th.p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user3.RemoteId = model.NewString(th.p.remoteID)
		user3, appErr = th.p.API.UpdateUser(user3)
		require.Nil(t, appErr)

		remoteUsers, err := th.p.MembersContainsRemote([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
			{UserId: user3.Id},
		})
		require.NoError(t, err)
		assert.True(t, remoteUsers)
	})

	t.Run("gm between two local users and a local user with teams as primary platform", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupUser(t, team)

		err := th.p.setPrimaryPlatform(user3.Id, storemodels.PreferenceValuePlatformMSTeams)
		require.NoError(t, err)

		_, appErr := th.p.API.GetGroupChannel([]string{user1.Id, user2.Id, user3.Id})
		require.Nil(t, appErr)

		remoteUsers, err := th.p.MembersContainsRemote([]*model.ChannelMember{
			{UserId: user1.Id},
			{UserId: user2.Id},
			{UserId: user3.Id},
		})
		require.NoError(t, err)
		assert.False(t, remoteUsers)
	})
}

func TestChannelConnectedOrRemote(t *testing.T) {
	th := setupTestHelper(t)

	t.Run("unknown sender id", func(t *testing.T) {
		remoteUsers, err := th.p.ChannelConnectedOrRemote("", model.NewId())
		require.Error(t, err)
		assert.False(t, remoteUsers)
	})

	t.Run("unknown channel id", func(t *testing.T) {
		remoteUsers, err := th.p.ChannelConnectedOrRemote(model.NewId(), "")
		require.Error(t, err)
		assert.False(t, remoteUsers)
	})

	t.Run("dm with a single users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)

		channel, appErr := th.p.API.GetDirectChannel(user1.Id, user1.Id)
		require.Nil(t, appErr)

		remoteUsers, err := th.p.ChannelConnectedOrRemote(channel.Id, user1.Id)
		require.NoError(t, err)
		assert.True(t, remoteUsers)
	})

	t.Run("dm between two local users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)
		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		channel, appErr := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, appErr)

		remoteUsers, err := th.p.ChannelConnectedOrRemote(channel.Id, user1.Id)
		require.NoError(t, err)
		assert.False(t, remoteUsers)
	})

	t.Run("dm between two remote users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupRemoteUser(t, team)

		var appErr *model.AppError

		user1.RemoteId = model.NewString(th.p.remoteID)
		user1, appErr = th.p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user2.RemoteId = model.NewString(th.p.remoteID)
		user2, appErr = th.p.API.UpdateUser(user2)
		require.Nil(t, appErr)

		channel, appErr := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, appErr)

		remoteUsers, err := th.p.ChannelConnectedOrRemote(channel.Id, user1.Id)
		require.NoError(t, err)
		assert.False(t, remoteUsers)
	})

	t.Run("dm between a local and a remote user", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		user2 := th.SetupRemoteUser(t, team)
		var appErr *model.AppError
		user2.RemoteId = model.NewString(th.p.remoteID)
		user2, appErr = th.p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		channel, appErr := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, appErr)

		remoteUsers, err := th.p.ChannelConnectedOrRemote(channel.Id, user1.Id)
		require.NoError(t, err)
		assert.True(t, remoteUsers)
	})

	t.Run("dm between a remote and a local user", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		var appErr *model.AppError
		user1.RemoteId = model.NewString(th.p.remoteID)
		user1, appErr = th.p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		channel, appErr := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, appErr)

		remoteUsers, err := th.p.ChannelConnectedOrRemote(channel.Id, user1.Id)
		require.NoError(t, err)
		assert.True(t, remoteUsers)
	})
}
