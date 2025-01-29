// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaybeSendInviteMessage(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	botUser, err := th.p.apiClient.User.Get(th.p.botUserID)
	require.NoError(t, err)

	tuesdayNoon, _ := time.Parse(time.RFC3339, "2024-01-09T12:00:00Z")
	saturdayEvening, _ := time.Parse(time.RFC3339, "2024-01-06T22:00:00Z")

	t.Run("don't send invite, invites disabled", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersMaxPendingInvites = 0
		})

		result, err := th.p.MaybeSendInviteMessage(user.Id, tuesdayNoon)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
		th.assertNoDMFromUser(t, botUser.Id, user.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
	})

	t.Run("don't send invite, max pending invites reached", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 0
			c.ConnectedUsersMaxPendingInvites = 1
		})

		result, err := th.p.MaybeSendInviteMessage(user.Id, tuesdayNoon)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
		th.assertNoDMFromUser(t, botUser.Id, user.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
	})

	t.Run("don't send invite, not whitelisted", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 1
			c.ConnectedUsersMaxPendingInvites = 1
			c.ConnectedUsersRestricted = true
		})

		result, err := th.p.MaybeSendInviteMessage(user.Id, tuesdayNoon)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
		th.assertNoDMFromUser(t, botUser.Id, user.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
	})

	t.Run("don't send invite, already invited", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 1
			c.ConnectedUsersMaxPendingInvites = 1
		})

		th.MarkUserInvited(t, user.Id)

		result, err := th.p.MaybeSendInviteMessage(user.Id, tuesdayNoon)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
		th.assertNoDMFromUser(t, botUser.Id, user.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
	})

	t.Run("don't send invite, already connected", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 2
			c.ConnectedUsersMaxPendingInvites = 1
		})

		th.ConnectUser(t, user.Id)

		result, err := th.p.MaybeSendInviteMessage(user.Id, tuesdayNoon)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
		th.assertNoDMFromUser(t, botUser.Id, user.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
	})

	t.Run("don't send invite, weekend", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 1
			c.ConnectedUsersMaxPendingInvites = 1
		})

		result, err := th.p.MaybeSendInviteMessage(user.Id, saturdayEvening)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
		th.assertNoDMFromUser(t, botUser.Id, user.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
	})

	t.Run("don't send invite, guest account", func(t *testing.T) {
		th.Reset(t)
		guestUser := th.SetupGuestUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 1
			c.ConnectedUsersMaxPendingInvites = 1
		})

		result, err := th.p.MaybeSendInviteMessage(guestUser.Id, tuesdayNoon)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
		th.assertNoDMFromUser(t, botUser.Id, guestUser.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
	})

	t.Run("don't send invite, bot account", func(t *testing.T) {
		th.Reset(t)
		otherBotUser := th.CreateBot(t)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 1
			c.ConnectedUsersMaxPendingInvites = 1
		})

		result, err := th.p.MaybeSendInviteMessage(otherBotUser.UserId, tuesdayNoon)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
		th.assertNoDMFromUser(t, botUser.Id, otherBotUser.UserId, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
	})

	t.Run("send invite, open invites allowed", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 1
			c.ConnectedUsersMaxPendingInvites = 1
		})

		result, err := th.p.MaybeSendInviteMessage(user.Id, tuesdayNoon)
		assert.NoError(t, err)
		assert.Equal(t, true, result)
		th.assertDMFromUserRe(t, botUser.Id, user.Id, "you've been invited by your administrator")
	})

	t.Run("send invite, whitelist restricted", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 1
			c.ConnectedUsersMaxPendingInvites = 1
			c.ConnectedUsersRestricted = true
		})

		th.MarkUserWhitelisted(t, user.Id)

		result, err := th.p.MaybeSendInviteMessage(user.Id, tuesdayNoon)
		assert.NoError(t, err)
		assert.Equal(t, true, result)
		th.assertDMFromUserRe(t, botUser.Id, user.Id, "you've been invited by your administrator")
	})
}

func TestCanInviteUser(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("cannot invite, invites disabled", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersMaxPendingInvites = 0
		})

		result, err := th.p.canInviteUser(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
	})

	t.Run("cannot invite, max invited reached", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)
		otherUser := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersMaxPendingInvites = 1
		})

		th.MarkUserInvited(t, otherUser.Id)

		result, err := th.p.canInviteUser(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
	})

	t.Run("cannot invite, whitelist restricted", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersMaxPendingInvites = 1
			c.ConnectedUsersRestricted = true
		})

		result, err := th.p.canInviteUser(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
	})

	t.Run("cannot invite, max connected reacted", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)
		otherUser := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 1
			c.ConnectedUsersMaxPendingInvites = 1
		})

		th.ConnectUser(t, otherUser.Id)

		result, err := th.p.canInviteUser(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
	})

	t.Run("can invite, whitelist restricted", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersMaxPendingInvites = 1
			c.ConnectedUsersRestricted = true
		})

		th.MarkUserWhitelisted(t, user.Id)

		result, err := th.p.canInviteUser(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, true, result)
	})
}

func TestUserHasRightToConnect(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("doesn't have right to connect by default", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		result, err := th.p.UserHasRightToConnect(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
	})

	t.Run("has right to connect, pending invite", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 0
		})

		th.MarkUserInvited(t, user.Id)

		result, err := th.p.UserHasRightToConnect(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("has right to connect, has connected before", func(t *testing.T) {
		th.Reset(t)
		user := th.SetupUser(t, team)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 0
		})

		th.ConnectUser(t, user.Id)
		th.DisconnectUser(t, user.Id)

		result, err := th.p.UserHasRightToConnect(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("does not have right to connect, is plugin bot", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 0
		})

		result, err := th.p.UserHasRightToConnect(th.p.botUserID)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
	})
}

func TestUserCanOpenlyConnect(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)
	user := th.SetupUser(t, team)
	otherUser := th.SetupUser(t, team)

	t.Run("cannot openly connect", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 0
		})

		result, nAvailable, err := th.p.UserCanOpenlyConnect(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
		assert.Equal(t, 0, nAvailable)
	})

	t.Run("can openly connect", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 1
		})

		result, nAvailable, err := th.p.UserCanOpenlyConnect(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, true, result)
		assert.Equal(t, 1, nAvailable)
	})

	t.Run("cannot openly connect, invite pool full", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 1
			c.ConnectedUsersMaxPendingInvites = 1
		})

		th.MarkUserInvited(t, otherUser.Id)

		result, nAvailable, err := th.p.UserCanOpenlyConnect(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
		assert.Equal(t, 0, nAvailable)
	})

	t.Run("cannot openly connect, whitelist restricted", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 1
			c.ConnectedUsersRestricted = true
		})

		result, nAvailable, err := th.p.UserCanOpenlyConnect(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, false, result)
		assert.Equal(t, 1, nAvailable)
	})

	t.Run("can openly connect, whitelist restricted", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 1
			c.ConnectedUsersRestricted = true
		})

		th.MarkUserWhitelisted(t, user.Id)

		result, nAvailable, err := th.p.UserCanOpenlyConnect(user.Id)
		assert.NoError(t, err)
		assert.Equal(t, true, result)
		assert.Equal(t, 1, nAvailable)
	})
}
