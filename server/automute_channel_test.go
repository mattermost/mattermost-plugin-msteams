package main

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestUpdateAutomutingOnUserJoinedChannel(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	setup := func(t *testing.T, automuteEnabled bool) (*Plugin, *model.User, *model.Channel, *model.Channel) {
		t.Helper()
		th.Reset(t)

		user := th.SetupUser(t, team)
		err := th.p.store.SetUserInfo(user.Id, "team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
		require.NoError(t, err)

		err = th.p.setAutomuteIsEnabledForUser(user.Id, automuteEnabled)
		require.NoError(t, err)

		linkedChannel := th.SetupPublicChannel(t, team)

		th.LinkChannel(t, team, linkedChannel, user)

		unlinkedChannel := th.SetupPublicChannel(t, team)

		return th.p, user, linkedChannel, unlinkedChannel
	}

	t.Run("when a user with automuting enabled joins a linked channel, the channel should be muted for that user", func(t *testing.T) {
		p, user, linkedChannel, _ := setup(t, true)

		_, appErr := p.API.AddUserToChannel(linkedChannel.Id, user.Id, user.Id)
		require.Nil(t, appErr)

		assertChannelAutomuted(t, th.p, linkedChannel.Id, user.Id)
	})

	t.Run("when a user without automuting enabled joins a linked channel, nothing should happen", func(t *testing.T) {
		p, user, linkedChannel, _ := setup(t, false)

		_, appErr := p.API.AddUserToChannel(linkedChannel.Id, user.Id, user.Id)
		require.Nil(t, appErr)

		assertChannelNotAutomuted(t, th.p, linkedChannel.Id, user.Id)
	})

	t.Run("when a user with automuting enabled joins a non-linked channel, nothing should happen", func(t *testing.T) {
		p, user, _, unlinkedChannel := setup(t, true)

		_, appErr := p.API.AddUserToChannel(unlinkedChannel.Id, user.Id, user.Id)
		require.Nil(t, appErr)

		assertChannelNotAutomuted(t, th.p, unlinkedChannel.Id, user.Id)
	})

	t.Run("when a user without automuting enabled joins a non-linked channel, nothing should happen", func(t *testing.T) {
		p, user, _, unlinkedChannel := setup(t, false)

		_, appErr := p.API.AddUserToChannel(unlinkedChannel.Id, user.Id, user.Id)
		require.Nil(t, appErr)

		assertChannelNotAutomuted(t, th.p, unlinkedChannel.Id, user.Id)
	})

	t.Run("should do nothing when an unconnected user joins a linked channel", func(t *testing.T) {
		p, _, linkedChannel, _ := setup(t, true)

		unconnectedUser := th.SetupUser(t, team)

		_, appErr := p.API.AddUserToChannel(linkedChannel.Id, unconnectedUser.Id, unconnectedUser.Id)
		require.Nil(t, appErr)

		assertChannelNotAutomuted(t, th.p, linkedChannel.Id, unconnectedUser.Id)
	})

	t.Run("should do nothing when an unconnected user joins an unlinked channel", func(t *testing.T) {
		p, _, _, unlinkedChannel := setup(t, true)

		unconnectedUser := th.SetupUser(t, team)

		_, appErr := p.API.AddUserToChannel(unlinkedChannel.Id, unconnectedUser.Id, unconnectedUser.Id)
		require.Nil(t, appErr)

		assertChannelNotAutomuted(t, th.p, unlinkedChannel.Id, unconnectedUser.Id)
	})

	t.Run("when a user with automuting enabled joins a linked channel, the channel should only be muted for them", func(t *testing.T) {
		p, user, linkedChannel, _ := setup(t, true)

		connectedUser := th.SetupUser(t, team)
		th.ConnectUser(t, connectedUser.Id)

		_, appErr := p.API.AddUserToChannel(linkedChannel.Id, connectedUser.Id, connectedUser.Id)
		require.Nil(t, appErr)

		unconnectedUser := th.SetupUser(t, team)

		_, appErr = p.API.AddUserToChannel(linkedChannel.Id, unconnectedUser.Id, connectedUser.Id)
		require.Nil(t, appErr)

		_, appErr = p.API.AddUserToChannel(linkedChannel.Id, user.Id, user.Id)
		require.Nil(t, appErr)

		assertChannelAutomuted(t, th.p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, th.p, linkedChannel.Id, connectedUser.Id)
		assertChannelNotAutomuted(t, th.p, linkedChannel.Id, unconnectedUser.Id)
	})
}

func TestUpdateAutomutingOnChannelCreated(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("when a DM is created, should mute it for users with automuting enabled", func(t *testing.T) {
		th.Reset(t)

		connectedUser := th.SetupUser(t, team)
		th.ConnectUser(t, connectedUser.Id)
		unconnectedUser := th.SetupUser(t, team)

		err := th.p.setAutomuteIsEnabledForUser(connectedUser.Id, true)
		require.NoError(t, err)

		channel, err := th.p.API.GetDirectChannel(connectedUser.Id, unconnectedUser.Id)
		require.Nil(t, err)

		assertChannelAutomuted(t, th.p, channel.Id, connectedUser.Id)
		assertChannelNotAutomuted(t, th.p, channel.Id, unconnectedUser.Id)
	})

	t.Run("when a GM is created, should mute it for users with automuting enabled", func(t *testing.T) {
		th.Reset(t)

		connectedUserWithAutomute := th.SetupUser(t, team)
		th.ConnectUser(t, connectedUserWithAutomute.Id)
		connectedUserWithoutAutomute := th.SetupUser(t, team)
		th.ConnectUser(t, connectedUserWithoutAutomute.Id)
		unconnectedUser := th.SetupUser(t, team)

		err := th.p.setAutomuteIsEnabledForUser(connectedUserWithAutomute.Id, true)
		require.NoError(t, err)

		channel, err := th.p.API.GetGroupChannel([]string{connectedUserWithAutomute.Id, connectedUserWithoutAutomute.Id, unconnectedUser.Id})
		require.Nil(t, err)

		assertChannelAutomuted(t, th.p, channel.Id, connectedUserWithAutomute.Id)
		assertChannelNotAutomuted(t, th.p, channel.Id, connectedUserWithoutAutomute.Id)
		assertChannelNotAutomuted(t, th.p, channel.Id, unconnectedUser.Id)
	})

	t.Run("when a regular channel is created, should do nothing", func(t *testing.T) {
		th.Reset(t)

		connectedUser := th.SetupUser(t, team)
		th.ConnectUser(t, connectedUser.Id)
		unconnectedUser := th.SetupUser(t, team)

		err := th.p.setAutomuteIsEnabledForUser(connectedUser.Id, true)
		require.NoError(t, err)

		channel := th.SetupPrivateChannel(t, team, WithMembers(connectedUser, unconnectedUser))

		assertChannelNotAutomuted(t, th.p, channel.Id, connectedUser.Id)
		assertChannelNotAutomuted(t, th.p, channel.Id, unconnectedUser.Id)
	})

	t.Run("when a DM is created, should not mute it for DMs with guest users", func(t *testing.T) {
		th.Reset(t)

		connectedUser := th.SetupUser(t, team)
		th.ConnectUser(t, connectedUser.Id)
		guestUser := th.SetupGuestUser(t, team)

		err := th.p.setAutomuteIsEnabledForUser(connectedUser.Id, true)
		require.NoError(t, err)

		channel, err := th.p.API.GetDirectChannel(connectedUser.Id, guestUser.Id)
		require.Nil(t, err)

		assertChannelNotAutomuted(t, th.p, channel.Id, connectedUser.Id)
		assertChannelNotAutomuted(t, th.p, channel.Id, guestUser.Id)
	})

	t.Run("when a DM is created, should not mute it for DMs with bots", func(t *testing.T) {
		th.Reset(t)

		connectedUser := th.SetupUser(t, team)
		th.ConnectUser(t, connectedUser.Id)
		bot := th.CreateBot(t)

		err := th.p.setAutomuteIsEnabledForUser(connectedUser.Id, true)
		require.NoError(t, err)

		channel, err := th.p.API.GetDirectChannel(connectedUser.Id, bot.UserId)
		require.Nil(t, err)

		assertChannelNotAutomuted(t, th.p, channel.Id, connectedUser.Id)
		assertChannelNotAutomuted(t, th.p, channel.Id, bot.UserId)
	})
}

func TestUpdateAutomutingOnChannelLinked(t *testing.T) {
	th := setupTestHelper(t)

	const (
		NotMuted = iota
		ManuallyMuted
		Automuted
	)

	for name, testCase := range map[string]struct {
		connected       bool
		automuteEnabled bool
		manuallyMuted   bool

		expect int
	}{
		"should not mute the channel for an unconnected user": {
			connected:     false,
			manuallyMuted: false,
			expect:        NotMuted,
		},
		"should leave the channel muted for an unconnected user": {
			connected:     false,
			manuallyMuted: true,
			expect:        ManuallyMuted,
		},
		"should not mute the channel for a connected user with automute disabled": {
			connected:       true,
			automuteEnabled: false,
			manuallyMuted:   false,
			expect:          NotMuted,
		},
		"should leave the channel muted for a connected user with automute disabled": {
			connected:       true,
			automuteEnabled: false,
			manuallyMuted:   true,
			expect:          ManuallyMuted,
		},
		"should mute the channel for a connected user with automute enabled": {
			connected:       true,
			automuteEnabled: true,
			manuallyMuted:   false,
			expect:          Automuted,
		},
		"should mute the channel for a connected user with automute enabled, overwriting manual muting": {
			connected:       true,
			automuteEnabled: true,
			manuallyMuted:   true,
			expect:          Automuted,
		},
	} {
		t.Run("when a channel is linked, "+name, func(t *testing.T) {
			th.Reset(t)

			team := th.SetupTeam(t)
			user := th.SetupUser(t, team)
			channel := th.SetupPublicChannel(t, team)

			if testCase.connected {
				th.ConnectUser(t, user.Id)
			}

			if testCase.automuteEnabled {
				err := th.p.setAutomuteIsEnabledForUser(user.Id, true)
				require.NoError(t, err)
			}

			_, appErr := th.p.API.AddUserToChannel(channel.Id, user.Id, user.Id)
			require.Nil(t, appErr)

			if testCase.manuallyMuted {
				appErr = th.p.API.PatchChannelMembersNotifications(
					[]*model.ChannelMemberIdentifier{
						{
							ChannelId: channel.Id,
							UserId:    user.Id,
						},
					},
					map[string]string{
						model.MarkUnreadNotifyProp: model.ChannelMarkUnreadMention,
					},
				)
				require.Nil(t, appErr)
			}

			// Ensure the channel starts correctly muted
			if testCase.manuallyMuted {
				assertChannelManuallyMuted(t, th.p, channel.Id, user.Id)
			} else {
				assertChannelNotAutomuted(t, th.p, channel.Id, user.Id)
			}

			err := th.p.updateAutomutingOnChannelLinked(channel.Id)
			require.NoError(t, err)

			// Confirm the channel was correctly muted or not
			switch testCase.expect {
			case NotMuted:
				assertChannelNotAutomuted(t, th.p, channel.Id, user.Id)

			case ManuallyMuted:
				assertChannelManuallyMuted(t, th.p, channel.Id, user.Id)

			case Automuted:
				assertChannelAutomuted(t, th.p, channel.Id, user.Id)
			}
		})
	}
}

func TestUpdateAutomutingOnChannelUnlinked(t *testing.T) {
	th := setupTestHelper(t)

	const (
		NotMuted = iota
		ManuallyMuted
		Automuted
	)

	for name, testCase := range map[string]struct {
		connected       bool
		automuteEnabled bool
		manuallyMuted   bool

		expect int
	}{
		"should not mute the channel for an unconnected user": {
			connected:     false,
			manuallyMuted: false,
			expect:        NotMuted,
		},
		"should leave the channel muted for an unconnected user": {
			connected:     false,
			manuallyMuted: true,
			expect:        ManuallyMuted,
		},
		"should not mute the channel for a connected user with automute disabled": {
			connected:       true,
			automuteEnabled: false,
			manuallyMuted:   false,
			expect:          NotMuted,
		},
		"should leave the channel muted for a connected user with automute disabled": {
			connected:       true,
			automuteEnabled: false,
			manuallyMuted:   true,
			expect:          ManuallyMuted,
		},
		"should unmute the channel for a connected user with automute enabled": {
			connected:       true,
			automuteEnabled: true,
			manuallyMuted:   false,
			expect:          NotMuted,
		},
		"should unmute the channel for a connected user with automute enabled, overwriting manual muting": {
			connected:       true,
			automuteEnabled: true,
			manuallyMuted:   true,
			expect:          NotMuted,
		},
	} {
		t.Run("when a channel is unlinked, "+name, func(t *testing.T) {
			th.Reset(t)

			team := th.SetupTeam(t)
			user := th.SetupUser(t, team)
			channel := th.SetupPublicChannel(t, team)

			th.LinkChannel(t, team, channel, user)

			if testCase.connected {
				th.ConnectUser(t, user.Id)
			}

			if testCase.automuteEnabled {
				err := th.p.setAutomuteIsEnabledForUser(user.Id, true)
				require.NoError(t, err)
			}

			_, appErr := th.p.API.AddUserToChannel(channel.Id, user.Id, user.Id)
			require.Nil(t, appErr)

			if testCase.manuallyMuted {
				appErr = th.p.API.PatchChannelMembersNotifications(
					[]*model.ChannelMemberIdentifier{
						{
							ChannelId: channel.Id,
							UserId:    user.Id,
						},
					},
					map[string]string{
						model.MarkUnreadNotifyProp: model.ChannelMarkUnreadMention,
					},
				)
				require.Nil(t, appErr)
			}

			if testCase.automuteEnabled {
				assertChannelAutomuted(t, th.p, channel.Id, user.Id)
			} else if testCase.manuallyMuted {
				assertChannelManuallyMuted(t, th.p, channel.Id, user.Id)
			} else {
				assertChannelNotAutomuted(t, th.p, channel.Id, user.Id)
			}

			err := th.p.updateAutomutingOnChannelUnlinked(channel.Id)
			require.NoError(t, err)

			// Confirm the channel was correctly unmuted or not
			switch testCase.expect {
			case NotMuted:
				assertChannelNotAutomuted(t, th.p, channel.Id, user.Id)

			case ManuallyMuted:
				assertChannelManuallyMuted(t, th.p, channel.Id, user.Id)

			case Automuted:
				assertChannelAutomuted(t, th.p, channel.Id, user.Id)
			}
		})
	}
}
