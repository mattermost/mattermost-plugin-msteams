// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"golang.org/x/oauth2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertNoCommandResponse(t *testing.T, actual *model.CommandResponse) {
	t.Helper()

	require.NotNil(t, actual)
	require.Equal(t, &model.CommandResponse{}, actual)
}

func assertWebsocketEvent(th *testHelper, t *testing.T, userID string, eventType string) {
	t.Helper()
	th.assertWebsocketEvent(t, userID, eventType)
}

func assertEphemeralResponse(th *testHelper, t *testing.T, args *model.CommandArgs, message string) {
	t.Helper()
	th.assertEphemeralMessage(t, args.UserId, args.ChannelId, message)
}

func TestExecuteDisconnectCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)

	th.SetupWebsocketClientForUser(t, user1.Id)

	t.Run("not connected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeDisconnectCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Error: the account is not connected")
	})

	t.Run("no token", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		err := th.p.store.SetUserInfo(user1.Id, "team_user_id", nil)
		require.NoError(t, err)

		commandResponse, appErr := th.p.executeDisconnectCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Error: the account is not connected")
	})

	t.Run("successfully disconnected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		err := th.p.store.SetUserInfo(user1.Id, "team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
		require.NoError(t, err)
		err = th.p.setNotificationPreference(user1.Id, true)
		require.NoError(t, err)

		commandResponse, appErr := th.p.executeDisconnectCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertWebsocketEvent(th, t, user1.Id, makePluginWebsocketEventName(WSEventUserDisconnected))
		assertEphemeralResponse(th, t, args, "Your account has been disconnected.")

		require.False(t, th.p.getNotificationPreference(user1.Id))
	})
}

func TestExecuteConnectCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)

	th.SetupWebsocketClientForUser(t, user1.Id)

	t.Run("already connected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		err := th.p.store.SetUserInfo(user1.Id, "team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
		require.NoError(t, err)

		commandResponse, appErr := th.p.executeConnectCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "You are already connected to MS Teams. Please disconnect your account first before connecting again.")
	})

	t.Run("not in whitelist, already at limit", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.ConnectedUsersAllowed = 0
		})

		commandResponse, appErr := th.p.executeConnectCommand(args)
		require.Nil(t, appErr)

		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "You cannot connect your account because the maximum limit of users allowed to connect has been reached. Please contact your system administrator.")
	})

	t.Run("successfully started connection", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeConnectCommand(args)
		require.Nil(t, appErr)

		assertNoCommandResponse(t, commandResponse)
		ephemeralPost := th.retrieveEphemeralPost(t, args.UserId, args.ChannelId)

		expectedMessage := fmt.Sprintf(`\[Click here to connect your account\]\(%s/connect\?post_id=(.*)&channel_id=%s&state_id=(.*)\)`, th.p.GetURL(), args.ChannelId)
		assert.Regexp(t, expectedMessage, ephemeralPost.Message)
	})
}

func TestGetAutocompleteData(t *testing.T) {
	for _, testCase := range []struct {
		description      string
		autocompleteData *model.AutocompleteData
	}{
		{
			description: "Successfully get all auto complete data",
			autocompleteData: &model.AutocompleteData{
				Trigger:   "msteams",
				Hint:      "[command]",
				HelpText:  "Manage MS Teams",
				RoleID:    model.SystemUserRoleId,
				Arguments: []*model.AutocompleteArg{},
				SubCommands: []*model.AutocompleteData{
					{
						Trigger:     "connect",
						HelpText:    "Connect your Mattermost account to your MS Teams account",
						RoleID:      model.SystemUserRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "disconnect",
						HelpText:    "Disconnect your Mattermost account from your MS Teams account",
						RoleID:      model.SystemUserRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "status",
						HelpText:    "Show your connection status",
						RoleID:      model.SystemUserRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:  "notifications",
						HelpText: "Enable or disable notifications from MSTeams. You must be connected to perform this action.",
						RoleID:   model.SystemUserRoleId,
						Arguments: []*model.AutocompleteArg{
							{
								Required: true,
								Type:     model.AutocompleteArgTypeStaticList,
								HelpText: "status",
								Data: &model.AutocompleteStaticListArg{
									PossibleArguments: []model.AutocompleteListItem{
										{
											Item:     "status",
											HelpText: "Show current notification status.",
										},
										{
											Item:     "on",
											HelpText: "Enable notifications from chats and group chats.",
										},
										{
											Item:     "off",
											HelpText: "Disable notifications from chats and group chats.",
										},
									},
								},
							},
						},
						SubCommands: []*model.AutocompleteData{},
					},
				},
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			autocompleteData := getAutocompleteData()
			assert.Equal(t, testCase.autocompleteData, autocompleteData)
		})
	}
}

func TestStatusCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)

	th.SetupWebsocketClientForUser(t, user1.Id)

	t.Run("not connected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeStatusCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Your account is not connected to Teams.")
	})

	t.Run("no token", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		err := th.p.store.SetUserInfo(user1.Id, "team_user_id", nil)
		require.NoError(t, err)

		commandResponse, appErr := th.p.executeStatusCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Your account is not connected to Teams.")
	})

	t.Run("connected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		err := th.p.store.SetUserInfo(user1.Id, "team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
		require.NoError(t, err)

		commandResponse, appErr := th.p.executeStatusCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Your account is connected to Teams.")
	})
}

func TestNotificationCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	args := &model.CommandArgs{
		UserId:    user1.Id,
		ChannelId: model.NewId(),
	}

	th.SetupWebsocketClientForUser(t, user1.Id)

	reset := func(th *testHelper, t *testing.T, connectUser bool) {
		t.Helper()
		th.Reset(t)

		if connectUser {
			th.ConnectUser(t, user1.Id)
		}

		err := th.p.API.DeletePreferencesForUser(user1.Id, []model.Preference{{
			UserId:   user1.Id,
			Category: PreferenceCategoryPlugin,
			Name:     storemodels.PreferenceNameNotification,
		}})
		require.Nil(t, err)
	}

	t.Run("not connected user should be rejected", func(t *testing.T) {
		reset(th, t, false)
		subCommands := []string{"status", "on", "off"}
		for _, subCommand := range subCommands {
			t.Run("subcommand "+subCommand, func(t *testing.T) {
				commandResponse, appErr := th.p.executeNotificationsCommand(args, []string{subCommand})
				require.Nil(t, appErr)
				assertNoCommandResponse(t, commandResponse)
				assertEphemeralResponse(th, t, args, "Error: Your account is not connected to Teams. To use this feature, please connect your account with `/msteams connect`.")
			})
		}
	})

	t.Run("status", func(t *testing.T) {
		t.Run("connected user should get the appropriate message", func(t *testing.T) {
			cases := []struct {
				name     string
				enabled  *bool
				expected string
			}{
				{
					name:     "enabled",
					enabled:  model.NewPointer(true),
					expected: "Notifications from chats and group chats in MS Teams are currently enabled.",
				},
				{
					name:     "disabled",
					enabled:  model.NewPointer(false),
					expected: "Notifications from chats and group chats in MS Teams are currently disabled.",
				},
				{
					name:     "not set",
					enabled:  nil,
					expected: "Notifications from chats and group chats in MS Teams are currently disabled.",
				},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					reset(th, t, true)
					if tc.enabled != nil {
						err := th.p.setNotificationPreference(user1.Id, *tc.enabled)
						require.Nil(t, err)
					}

					commandResponse, appErr := th.p.executeNotificationsCommand(args, []string{"status"})
					require.Nil(t, appErr)
					assertNoCommandResponse(t, commandResponse)
					assertEphemeralResponse(th, t, args, tc.expected)
				})
			}
		})
	})

	t.Run("on", func(t *testing.T) {
		reset(th, t, true)

		cases := []struct {
			name    string
			enabled *bool
		}{
			{name: "was enabled", enabled: model.NewPointer(true)},
			{name: "was disabled", enabled: model.NewPointer(false)},
			{name: "was not set", enabled: nil},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.enabled != nil {
					err := th.p.setNotificationPreference(user1.Id, *tc.enabled)
					require.Nil(t, err)
				}

				commandResponse, appErr := th.p.executeNotificationsCommand(args, []string{"on"})
				require.Nil(t, appErr)
				assertNoCommandResponse(t, commandResponse)
				assertEphemeralResponse(th, t, args, "Notifications from chats and group chats in MS Teams are now enabled.")

				require.True(t, th.p.getNotificationPreference(user1.Id))
			})
		}
	})

	t.Run("off", func(t *testing.T) {
		reset(th, t, true)

		cases := []struct {
			name    string
			enabled *bool
		}{
			{name: "was enabled", enabled: model.NewPointer(true)},
			{name: "was disabled", enabled: model.NewPointer(false)},
			{name: "was not set", enabled: nil},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.enabled != nil {
					err := th.p.setNotificationPreference(user1.Id, *tc.enabled)
					require.Nil(t, err)
				}

				commandResponse, appErr := th.p.executeNotificationsCommand(args, []string{"off"})
				require.Nil(t, appErr)
				assertNoCommandResponse(t, commandResponse)
				assertEphemeralResponse(th, t, args, "Notifications from chats and group chats in MS Teams are now disabled.")

				require.False(t, th.p.getNotificationPreference(user1.Id))
			})
		}
	})
}
