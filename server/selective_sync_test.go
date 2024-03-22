//go:build exclude

package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	goPlugin "github.com/hashicorp/go-plugin"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPlugin(t *testing.T) *Plugin {
	t.Helper()

	p := &Plugin{}
	p.remoteID = "remote-id"

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	reattachConfigCh := make(chan *goPlugin.ReattachConfig)

	go plugin.ClientMainTesting(ctx, p, reattachConfigCh)

	var reattachConfig *goPlugin.ReattachConfig
	select {
	case reattachConfig = <-reattachConfigCh:
	case <-time.After(5 * time.Second):
		t.Fatal("failed to get reattach config")
	}

	socketPath := os.Getenv("MM_LOCALSOCKETPATH")
	if socketPath == "" {
		socketPath = model.LocalModeSocketPath
	}

	clientLocal := model.NewAPIv4SocketClient(socketPath)
	_, err := clientLocal.ReattachPlugin(ctx, &model.PluginReattachRequest{
		Manifest:             manifest,
		PluginReattachConfig: model.NewPluginReattachConfig(reattachConfig),
	})
	require.NoError(t, err)

	return p
}

func setupTeam(t *testing.T, p *Plugin) *model.Team {
	t.Helper()

	teamName := model.NewRandomTeamName()
	team, appErr := p.API.CreateTeam(&model.Team{
		Name:        teamName,
		DisplayName: teamName,
		Type:        model.TeamOpen,
	})
	require.Nil(t, appErr)

	return team
}
func setupUser(t *testing.T, p *Plugin, team *model.Team, isRemote bool) *model.User {
	t.Helper()

	username := model.NewId()
	if isRemote {
		username = fmt.Sprintf("msteams_%s", username)
	}

	user := &model.User{
		Email:         fmt.Sprintf("%s@example.com", username),
		Username:      username,
		Password:      "password",
		EmailVerified: true,
	}

	user, appErr := p.API.CreateUser(user)
	require.Nil(t, appErr)

	_, appErr = p.API.AddTeamMembers(team.Id, []string{user.Id}, "", false)
	require.Nil(t, appErr)

	return user
}

func TestChatSpansPlatforms(t *testing.T) {
	t.Skip("pending upstream changes: https://mattermost.atlassian.net/browse/MM-57018")

	p := setupPlugin(t)

	t.Run("invalid channel id", func(t *testing.T) {
		_, appErr := p.ChatSpansPlatforms("")
		require.Error(t, appErr)
	})

	t.Run("unknown channel id", func(t *testing.T) {
		_, appErr := p.ChatSpansPlatforms(model.NewId())
		require.Error(t, appErr)
	})

	t.Run("dm between two local users", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, false)
		user2 := setupUser(t, p, team, false)

		channel, err := p.API.CreateDirectChannel(user1.Id, user2.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
	})

	t.Run("dm between two remote users", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, true)
		user2 := setupUser(t, p, team, true)

		var appErr *model.AppError

		user1.RemoteId = model.NewString("remote-id")
		user1, appErr = p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user2.RemoteId = model.NewString("remote-id")
		user2, appErr = p.API.UpdateUser(user2)
		require.Nil(t, appErr)

		channel, err := p.API.CreateDirectChannel(user1.Id, user2.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
	})

	t.Run("dm between a local and a remote user", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, true)
		user2 := setupUser(t, p, team, false)

		var appErr *model.AppError

		user1.RemoteId = model.NewString("remote-id")
		user1, appErr = p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		channel, err := p.API.CreateDirectChannel(user1.Id, user2.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("dm between a local and a local user with teams as primary platform", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, false)
		user2 := setupUser(t, p, team, false)

		var appErr *model.AppError

		p.setAutomuteIsEnabledForUser(user2.Id, true)

		channel, err := p.API.CreateDirectChannel(user1.Id, user2.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("gm between three local users", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, false)
		user2 := setupUser(t, p, team, false)
		user3 := setupUser(t, p, team, false)

		channel, err := p.API.CreateGroupChannel([]string{user1.Id, user2.Id, user3.Id}, user1.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
	})

	t.Run("gm between three remote users", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, true)
		user2 := setupUser(t, p, team, true)
		user3 := setupUser(t, p, team, true)

		var appErr *model.AppError

		user1.RemoteId = model.NewString("remote-id")
		user1, appErr = p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user2.RemoteId = model.NewString("remote-id")
		user2, appErr = p.API.UpdateUser(user2)
		require.Nil(t, appErr)

		user3.RemoteId = model.NewString("remote-id")
		user3, appErr = p.API.UpdateUser(user3)
		require.Nil(t, appErr)

		channel, err := p.API.CreateGroupChannel([]string{user1.Id, user2.Id, user3.Id}, user1.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
	})

	t.Run("gm between a mixture of local and remote users", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, true)
		user2 := setupUser(t, p, team, false)
		user3 := setupUser(t, p, team, true)

		var appErr *model.AppError

		user1.RemoteId = model.NewString("remote-id")
		user1, appErr = p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user3.RemoteId = model.NewString("remote-id")
		user3, appErr = p.API.UpdateUser(user3)
		require.Nil(t, appErr)

		channel, err := p.API.CreateGroupChannel([]string{user1.Id, user2.Id, user3.Id}, user1.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("gm between two local users and a local user with teams as primary platform", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, false)
		user2 := setupUser(t, p, team, false)
		user3 := setupUser(t, p, team, false)

		var appErr *model.AppError

		p.setAutomuteIsEnabledForUser(user3.Id, true)

		channel, err := p.API.CreateGroupChannel([]string{user1.Id, user2.Id, user3.Id}, user1.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := p.ChatSpansPlatforms(channel.Id)
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})
}

func TestChatMembersSpanPlatforms(t *testing.T) {
	t.Skip("pending upstream changes: https://mattermost.atlassian.net/browse/MM-57018")

	p := setupPlugin(t)

	t.Run("empty set of channel members", func(t *testing.T) {
		chatMembersSpanPlatforms, appErr := p.ChatMembersSpanPlatforms(model.ChannelMembers{})
		require.Nil(t, appErr)
		require.False(t, chatMembersSpanPlatforms)
	})

	t.Run("user with empty id", func(t *testing.T) {
		chatMembersSpanPlatforms, appErr := p.ChatMembersSpanPlatforms(model.ChannelMembers{model.ChannelMember{UserId: ""}})
		require.Error(t, appErr)
		require.False(t, chatMembersSpanPlatforms)
	})

	t.Run("single local user", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, false)

		chatSpansPlatforms, appErr := p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
		})

		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("dm between two local users", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, false)
		user2 := setupUser(t, p, team, false)

		chatSpansPlatforms, appErr := p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
		})
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
	})

	t.Run("dm between two remote users", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, true)
		user2 := setupUser(t, p, team, true)

		var appErr *model.AppError

		user1.RemoteId = model.NewString("remote-id")
		user1, appErr = p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user2.RemoteId = model.NewString("remote-id")
		user2, appErr = p.API.UpdateUser(user2)
		require.Nil(t, appErr)

		chatSpansPlatforms, appErr := p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
		})
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
	})

	t.Run("dm between a local and a remote user", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, true)
		user2 := setupUser(t, p, team, false)

		var appErr *model.AppError

		user1.RemoteId = model.NewString("remote-id")
		user1, appErr = p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		chatSpansPlatforms, appErr := p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
		})
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("dm between a local and a local user with teams as primary platform", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, false)
		user2 := setupUser(t, p, team, false)

		var appErr *model.AppError

		p.setAutomuteIsEnabledForUser(user2.Id, true)

		_, err := p.API.CreateDirectChannel(user1.Id, user2.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
		})
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("gm between three local users", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, false)
		user2 := setupUser(t, p, team, false)
		user3 := setupUser(t, p, team, false)

		chatSpansPlatforms, appErr := p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
			model.ChannelMember{UserId: user3.Id},
		})
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
	})

	t.Run("gm between three remote users", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, true)
		user2 := setupUser(t, p, team, true)
		user3 := setupUser(t, p, team, true)

		var appErr *model.AppError

		user1.RemoteId = model.NewString("remote-id")
		user1, appErr = p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user2.RemoteId = model.NewString("remote-id")
		user2, appErr = p.API.UpdateUser(user2)
		require.Nil(t, appErr)

		user3.RemoteId = model.NewString("remote-id")
		user3, appErr = p.API.UpdateUser(user3)
		require.Nil(t, appErr)

		chatSpansPlatforms, appErr := p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
			model.ChannelMember{UserId: user3.Id},
		})
		require.Nil(t, appErr)
		assert.False(t, chatSpansPlatforms)
	})

	t.Run("gm between a mixture of local and remote users", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, true)
		user2 := setupUser(t, p, team, false)
		user3 := setupUser(t, p, team, true)

		var appErr *model.AppError

		user1.RemoteId = model.NewString("remote-id")
		user1, appErr = p.API.UpdateUser(user1)
		require.Nil(t, appErr)

		user3.RemoteId = model.NewString("remote-id")
		user3, appErr = p.API.UpdateUser(user3)
		require.Nil(t, appErr)

		chatSpansPlatforms, appErr := p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
			model.ChannelMember{UserId: user3.Id},
		})
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})

	t.Run("gm between two local users and a local user with teams as primary platform", func(t *testing.T) {
		team := setupTeam(t, p)
		user1 := setupUser(t, p, team, false)
		user2 := setupUser(t, p, team, false)
		user3 := setupUser(t, p, team, false)

		var appErr *model.AppError

		p.setAutomuteIsEnabledForUser(user3.Id, true)

		_, err := p.API.CreateGroupChannel([]string{user1.Id, user2.Id, user3.Id}, user1.Id)
		require.Nil(t, err)

		chatSpansPlatforms, appErr := p.ChatMembersSpanPlatforms(model.ChannelMembers{
			model.ChannelMember{UserId: user1.Id},
			model.ChannelMember{UserId: user2.Id},
			model.ChannelMember{UserId: user3.Id},
		})
		require.Nil(t, appErr)
		assert.True(t, chatSpansPlatforms)
	})
}
