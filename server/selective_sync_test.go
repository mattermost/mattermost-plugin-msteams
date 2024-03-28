package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatSpansPlatforms(t *testing.T) {
	th := setupTestHelper(t)

	t.Run("invalid channel id", func(t *testing.T) {
		_, appErr := th.p.ChatSpansPlatforms("")
		require.Error(t, appErr)
	})

	t.Run("unknown channel id", func(t *testing.T) {
		_, appErr := th.p.ChatSpansPlatforms(model.NewId())
		require.Error(t, appErr)
	})

	t.Run("dm between two local users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)

		channel, err := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := th.p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
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

		channel, err := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := th.p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
	})

	t.Run("dm between a local and a remote user", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupUser(t, team)

		var appErr *model.AppError

		user1.RemoteId = model.NewString(th.p.remoteID)
		user1, appErr = th.p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		channel, err := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := th.p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("dm between a local and a local user with teams as primary platform", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)

		err := th.p.setPrimaryPlatform(user2.Id, PreferenceValuePlatformMSTeams)
		require.NoError(t, err)

		channel, err := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := th.p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("gm between three local users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupUser(t, team)

		channel, err := th.p.API.GetGroupChannel([]string{user1.Id, user2.Id, user3.Id})
		require.Nil(t, err)

		chatSpansPlatforms, appErr := th.p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
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

		channel, err := th.p.API.GetGroupChannel([]string{user1.Id, user2.Id, user3.Id})
		require.Nil(t, err)

		chatSpansPlatforms, appErr := th.p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
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

		channel, err := th.p.API.GetGroupChannel([]string{user1.Id, user2.Id, user3.Id})
		require.Nil(t, err)

		chatSpansPlatforms, appErr := th.p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("gm between two local users and a local user with teams as primary platform", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupUser(t, team)

		err := th.p.setPrimaryPlatform(user3.Id, PreferenceValuePlatformMSTeams)
		require.NoError(t, err)

		channel, appErr := th.p.API.GetGroupChannel([]string{user1.Id, user2.Id, user3.Id})
		require.Nil(t, appErr)

		chatSpansPlatforms, appErr := th.p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})
}

func TestChatMembersSpanPlatforms(t *testing.T) {
	th := setupTestHelper(t)

	t.Run("empty set of channel members", func(t *testing.T) {
		chatMembersSpanPlatforms, appErr := th.p.ChatMembersSpanPlatforms(model.ChannelMembers{})
		require.Nil(t, appErr)
		require.False(t, chatMembersSpanPlatforms)
	})

	t.Run("users with empty id", func(t *testing.T) {
		chatMembersSpanPlatforms, appErr := th.p.ChatMembersSpanPlatforms(model.ChannelMembers{model.ChannelMember{UserId: ""}, model.ChannelMember{UserId: ""}})
		require.NotNil(t, appErr)
		require.False(t, chatMembersSpanPlatforms)
	})

	t.Run("single local user", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)

		chatSpansPlatforms, appErr := th.p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
		})

		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("dm between two local users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)

		chatSpansPlatforms, appErr := th.p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
		})
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
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

		chatSpansPlatforms, appErr := th.p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
		})
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
	})

	t.Run("dm between a local and a remote user", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupRemoteUser(t, team)
		user2 := th.SetupUser(t, team)

		var appErr *model.AppError

		user1.RemoteId = model.NewString(th.p.remoteID)
		user1, appErr = th.p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		chatSpansPlatforms, appErr := th.p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
		})
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("dm between a local and a local user with teams as primary platform", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)

		err := th.p.setPrimaryPlatform(user2.Id, PreferenceValuePlatformMSTeams)
		require.NoError(t, err)

		_, appErr := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, appErr)

		chatSpansPlatforms, appErr := th.p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
		})
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("gm between three local users", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupUser(t, team)

		chatSpansPlatforms, appErr := th.p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
			model.ChannelMember{UserId: user3.Id},
		})
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
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

		chatSpansPlatforms, appErr := th.p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
			model.ChannelMember{UserId: user3.Id},
		})
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
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

		chatSpansPlatforms, appErr := th.p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
			model.ChannelMember{UserId: user3.Id},
		})
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("gm between two local users and a local user with teams as primary platform", func(t *testing.T) {
		team := th.SetupTeam(t)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)
		user3 := th.SetupUser(t, team)

		err := th.p.setPrimaryPlatform(user3.Id, PreferenceValuePlatformMSTeams)
		require.NoError(t, err)

		_, appErr := th.p.API.GetGroupChannel([]string{user1.Id, user2.Id, user3.Id})
		require.Nil(t, appErr)

		chatSpansPlatforms, appErr := th.p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
			model.ChannelMember{UserId: user3.Id},
		})
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})
}
