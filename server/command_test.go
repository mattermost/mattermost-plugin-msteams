package main

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestExecuteUnlinkCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	user2 := th.SetupUser(t, team)
	user3 := th.SetupUser(t, team)

	th.SetupWebsocketClientForUser(t, user1.Id)

	t.Run("invalid channel", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = true

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}
		commandResponse, appErr := th.p.executeUnlinkCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Unable to get the current channel information.")
	})

	t.Run("channel is a dm", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = true
		channel, err := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, err)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}
		commandResponse, appErr := th.p.executeUnlinkCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Linking/unlinking a direct or group message is not allowed")
	})

	t.Run("channel is a gm", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = true
		channel, err := th.p.API.GetGroupChannel([]string{user1.Id, user2.Id, user3.Id})
		require.Nil(t, err)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}
		commandResponse, appErr := th.p.executeUnlinkCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Linking/unlinking a direct or group message is not allowed")
	})

	t.Run("not a channel admin", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = true
		channel := th.SetupPublicChannel(t, team)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}
		commandResponse, appErr := th.p.executeUnlinkCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Unable to unlink the channel, you have to be a channel admin to unlink it.")
	})

	t.Run("not a linked channel", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = true
		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}
		commandResponse, appErr := th.p.executeUnlinkCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "This Mattermost channel is not linked to any MS Teams channel.")
	})

	t.Run("successfully unlinked", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = true
		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		channelLink := storemodels.ChannelLink{
			MattermostTeamID:    team.Id,
			MattermostChannelID: channel.Id,
			MSTeamsTeam:         model.NewId(),
			MSTeamsChannel:      model.NewId(),
			Creator:             user1.Id,
		}

		err := th.p.store.StoreChannelLink(&channelLink)
		require.NoError(t, err)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}
		commandResponse, appErr := th.p.executeUnlinkCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "The MS Teams channel is no longer linked to this Mattermost channel.")
	})

	t.Run("successfully unlinked, sync msg enabled", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = false
		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		channelLink := storemodels.ChannelLink{
			MattermostTeamID:    team.Id,
			MattermostChannelID: channel.Id,
			MSTeamsTeam:         model.NewId(),
			MSTeamsChannel:      model.NewId(),
			Creator:             user1.Id,
		}

		err := th.p.store.StoreChannelLink(&channelLink)
		require.NoError(t, err)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}
		commandResponse, appErr := th.p.executeUnlinkCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "The MS Teams channel is no longer linked to this Mattermost channel.")
	})
}

func TestExecuteShowCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)

	th.SetupWebsocketClientForUser(t, user1.Id)

	t.Run("invalid channel id", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: "",
		}

		commandResponse, appErr := th.p.executeShowCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Link doesn't exist.")
	})

	t.Run("unlinked channel", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: "unlinked",
		}

		commandResponse, appErr := th.p.executeShowCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Link doesn't exist.")
	})

	t.Run("unable to get the MS Teams team", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		// Setup a link in the database
		err := th.p.store.StoreChannelLink(&storemodels.ChannelLink{
			MattermostTeamID:      model.NewId(),
			MattermostTeamName:    "team_name",
			MattermostChannelID:   args.ChannelId,
			MattermostChannelName: "channel_name",
			MSTeamsTeam:           "invalid_team",
			MSTeamsChannel:        "invalid_channel",
			Creator:               "creator",
		})
		require.NoError(t, err)

		th.appClientMock.On("GetTeam", "invalid_team").Return(nil, fmt.Errorf("no such team")).Times(1)

		commandResponse, appErr := th.p.executeShowCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Unable to get the MS Teams team information.")
	})

	t.Run("unable to get the MS Teams channel", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		// Setup a link in the database
		err := th.p.store.StoreChannelLink(&storemodels.ChannelLink{
			MattermostTeamID:      model.NewId(),
			MattermostTeamName:    "team_name",
			MattermostChannelID:   args.ChannelId,
			MattermostChannelName: "channel_name",
			MSTeamsTeam:           "team",
			MSTeamsChannel:        "invalid_channel",
			Creator:               "creator",
		})
		require.NoError(t, err)

		th.appClientMock.On("GetTeam", "team").Return(&clientmodels.Team{DisplayName: "Team"}, nil).Times(1)
		th.appClientMock.On("GetChannelInTeam", "team", "invalid_channel").Return(nil, fmt.Errorf("no such channel")).Times(1)

		commandResponse, appErr := th.p.executeShowCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Unable to get the MS Teams channel information.")
	})

	t.Run("successfully executed show command", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		// Setup a link in the database
		err := th.p.store.StoreChannelLink(&storemodels.ChannelLink{
			MattermostTeamID:      model.NewId(),
			MattermostTeamName:    "team_name",
			MattermostChannelID:   args.ChannelId,
			MattermostChannelName: "channel_name",
			MSTeamsTeam:           "team",
			MSTeamsChannel:        "channel",
			Creator:               "creator",
		})
		require.NoError(t, err)

		th.appClientMock.On("GetTeam", "team").Return(&clientmodels.Team{DisplayName: "Team"}, nil).Times(1)
		th.appClientMock.On("GetChannelInTeam", "team", "channel").Return(&clientmodels.Channel{DisplayName: "Channel"}, nil).Times(1)

		commandResponse, appErr := th.p.executeShowCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "This channel is linked to the MS Teams Channel \"Channel\" in the Team \"Team\".")
	})
}

func TestExecuteShowLinksCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	channel1 := th.SetupPublicChannel(t, team)
	user1 := th.SetupUser(t, team)
	sysadmin1 := th.SetupSysadmin(t, team)

	th.SetupWebsocketClientForUser(t, user1.Id)
	th.SetupWebsocketClientForUser(t, sysadmin1.Id)

	t.Run("not a system admin", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeShowLinksCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Unable to execute the command, only system admins have access to execute this command.")
	})

	t.Run("no links", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: "unlinked",
		}

		commandResponse, appErr := th.p.executeShowLinksCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "No links present.")
	})

	t.Run("failed fetching teams info", func(t *testing.T) {
		th.Reset(t)

		// Setup a single link in the database
		err := th.p.store.StoreChannelLink(&storemodels.ChannelLink{
			MattermostTeamID:      team.Id,
			MattermostTeamName:    "team_name",
			MattermostChannelID:   channel1.Id,
			MattermostChannelName: "channel_name",
			MSTeamsTeam:           "team",
			MSTeamsChannel:        "channel",
			Creator:               "creator",
		})
		require.NoError(t, err)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: "unlinked",
		}

		th.appClientMock.On("GetTeams", "id in ('team')").Return(nil, fmt.Errorf("an error occurred")).Times(1)
		th.appClientMock.On("GetChannelsInTeam", "team", "id in ('channel')").Return([]*clientmodels.Channel{{ID: "channel", DisplayName: "Teams Channel"}}, nil).Times(1)

		commandResponse, appErr := th.p.executeShowLinksCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)

		assertEphemeralResponse(th, t, args, "Please wait while your request is being processed.")
		assertEphemeralResponse(th, t, args, fmt.Sprintf(`| Mattermost Team | Mattermost Channel | MS Teams Team | MS Teams Channel |
| :------|:--------|:-------|:-----------|
|%s|%s||Teams Channel|
There were some errors while fetching information. Please check the server logs.`, team.DisplayName, channel1.DisplayName))
	})

	t.Run("one link", func(t *testing.T) {
		th.Reset(t)

		// Setup a single link in the database
		err := th.p.store.StoreChannelLink(&storemodels.ChannelLink{
			MattermostTeamID:      team.Id,
			MattermostTeamName:    "team_name",
			MattermostChannelID:   channel1.Id,
			MattermostChannelName: "channel_name",
			MSTeamsTeam:           "team",
			MSTeamsChannel:        "channel",
			Creator:               "creator",
		})
		require.NoError(t, err)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: "unlinked",
		}

		th.appClientMock.On("GetTeams", "id in ('team')").Return([]*clientmodels.Team{{ID: "team", DisplayName: "Teams Team"}}, nil).Times(1)
		th.appClientMock.On("GetChannelsInTeam", "team", "id in ('channel')").Return([]*clientmodels.Channel{{ID: "channel", DisplayName: "Teams Channel"}}, nil).Times(1)

		commandResponse, appErr := th.p.executeShowLinksCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)

		assertEphemeralResponse(th, t, args, "Please wait while your request is being processed.")
		assertEphemeralResponse(th, t, args, fmt.Sprintf(`| Mattermost Team | Mattermost Channel | MS Teams Team | MS Teams Channel |
| :------|:--------|:-------|:-----------|
|%s|%s|Teams Team|Teams Channel|`, team.DisplayName, channel1.DisplayName))
	})
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
		err = th.p.setPrimaryPlatform(user1.Id, storemodels.PreferenceValuePlatformMSTeams)
		require.NoError(t, err)
		err = th.p.setNotificationPreference(user1.Id, true)
		require.NoError(t, err)

		commandResponse, appErr := th.p.executeDisconnectCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertWebsocketEvent(th, t, user1.Id, makePluginWebsocketEventName(WSEventUserDisconnected))
		assertEphemeralResponse(th, t, args, "Your account has been disconnected.")

		require.Equal(t, storemodels.PreferenceValuePlatformMM, th.p.getPrimaryPlatform(user1.Id))
		require.Equal(t, false, th.p.getNotificationPreference(user1.Id))
	})
}

func TestExecuteDisconnectBotCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	sysadmin1 := th.SetupSysadmin(t, team)

	th.SetupWebsocketClientForUser(t, user1.Id)
	th.SetupWebsocketClientForUser(t, sysadmin1.Id)

	t.Run("not a system admin", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeDisconnectBotCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Unable to disconnect the bot account, only system admins can disconnect the bot account.")
	})

	t.Run("bot user is not connected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeDisconnectBotCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Error: unable to find the connected bot account")
	})

	t.Run("successfully disconnected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}

		err := th.p.store.SetUserInfo(th.p.botUserID, "bot_team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})

		require.NoError(t, err)

		commandResponse, appErr := th.p.executeDisconnectBotCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "The bot account has been disconnected.")
	})
}

func TestExecuteLinkCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	user2 := th.SetupUser(t, team)
	user3 := th.SetupUser(t, team)

	th.SetupWebsocketClientForUser(t, user1.Id)

	t.Run("unknown channel", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{})
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Unable to get the current channel information.")
	})

	t.Run("direct message", func(t *testing.T) {
		th.Reset(t)

		channel, err := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, err)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{})
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Linking/unlinking a direct or group message is not allowed")
	})

	t.Run("group message", func(t *testing.T) {
		th.Reset(t)

		channel, err := th.p.API.GetGroupChannel([]string{user1.Id, user2.Id, user3.Id})
		require.Nil(t, err)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{})
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Linking/unlinking a direct or group message is not allowed")
	})

	t.Run("not a channel admin", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{})
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Unable to link the channel. You have to be a channel admin to link it.")
	})

	t.Run("missing parameters", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{})
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Invalid link command, please pass the MS Teams team id and channel id as parameters.")
	})

	t.Run("linking channel not allowed in this team", func(t *testing.T) {
		t.Skip("not yet implemented")
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{"msteams_team_id", "msteams_channel_id"})
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "This team is not enabled for MS Teams sync.")
	})

	t.Run("already linked", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		channelLink := storemodels.ChannelLink{
			MattermostTeamID:    team.Id,
			MattermostChannelID: channel.Id,
			MSTeamsTeam:         model.NewId(),
			MSTeamsChannel:      model.NewId(),
			Creator:             user1.Id,
		}

		err := th.p.store.StoreChannelLink(&channelLink)
		require.NoError(t, err)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{channelLink.MSTeamsTeam, channelLink.MSTeamsChannel})
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "A link for this channel already exists. Please unlink the channel before you link again with another channel.")
	})

	t.Run("another channel already linked", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		otherChannel := th.SetupPublicChannel(t, team)

		channelLink := storemodels.ChannelLink{
			MattermostTeamID:    team.Id,
			MattermostChannelID: otherChannel.Id,
			MSTeamsTeam:         model.NewId(),
			MSTeamsChannel:      model.NewId(),
			Creator:             user1.Id,
		}

		err := th.p.store.StoreChannelLink(&channelLink)
		require.NoError(t, err)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{channelLink.MSTeamsTeam, channelLink.MSTeamsChannel})
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "A link for this channel already exists. Please unlink the channel before you link again with another channel.")
	})

	t.Run("user not connected", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{"msteams_team_id", "msteams_channel_id"})
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Unable to link the channel, looks like your account is not connected to MS Teams")
	})

	t.Run("not found on Teams", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		err := th.p.store.SetUserInfo(user1.Id, "team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
		require.NoError(t, err)

		th.clientMock.On("GetChannelInTeam", "msteams_team_id", "msteams_channel_id").Return(nil, fmt.Errorf("no such channel")).Times(1)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{"msteams_team_id", "msteams_channel_id"})
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "MS Teams channel not found or you don't have the permissions to access it.")
	})

	t.Run("failed to subscribe", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		err := th.p.store.SetUserInfo(user1.Id, "team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
		require.NoError(t, err)

		th.clientMock.On("GetChannelInTeam", "msteams_team_id", "msteams_channel_id").Return(&clientmodels.Channel{}, nil).Times(1)
		th.appClientMock.
			On("SubscribeToChannel", "msteams_team_id", "msteams_channel_id", mock.AnythingOfType("string"), mock.AnythingOfType("string"), "").
			Return(nil, errors.New("an error occurred")).
			Times(1)

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{"msteams_team_id", "msteams_channel_id"})
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)

		assertEphemeralResponse(th, t, args, commandWaitingMessage)
		assertEphemeralResponse(th, t, args, "Unable to subscribe to the channel")
	})

	t.Run("successfully linked", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		err := th.p.store.SetUserInfo(user1.Id, "team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
		require.NoError(t, err)

		th.clientMock.On("GetChannelInTeam", "msteams_team_id", "msteams_channel_id").Return(&clientmodels.Channel{}, nil).Times(1)
		th.appClientMock.
			On("SubscribeToChannel", "msteams_team_id", "msteams_channel_id", mock.AnythingOfType("string"), mock.AnythingOfType("string"), "").
			Return(&clientmodels.Subscription{ID: model.NewId()}, nil).
			Times(1)

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{"msteams_team_id", "msteams_channel_id"})
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, commandWaitingMessage)
		assertEphemeralResponse(th, t, args, "The MS Teams channel is now linked to this Mattermost channel.")
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

		configuration := th.p.configuration.Clone()
		configuration.ConnectedUsersAllowed = 0
		th.p.setConfiguration(configuration)
		defer func() {
			configuration := th.p.configuration.Clone()
			configuration.ConnectedUsersAllowed = 1000
			th.p.setConfiguration(configuration)
		}()

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

		expectedMessage := fmt.Sprintf(`\[Click here to connect your account\]\(%s/connect\?post_id=(.*)&channel_id=%s\)`, th.p.GetURL(), args.ChannelId)
		result, _ := regexp.MatchString(expectedMessage, ephemeralPost.Message)
		assert.True(t, result)
	})
}

func TestExecuteConnectBotCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	sysadmin1 := th.SetupSysadmin(t, team)

	th.SetupWebsocketClientForUser(t, user1.Id)
	th.SetupWebsocketClientForUser(t, sysadmin1.Id)

	t.Run("not a system admin", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeConnectBotCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Unable to connect the bot account, only system admins can connect the bot account.")
	})

	t.Run("bot user already connected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}

		err := th.p.store.SetUserInfo(th.p.botUserID, "bot_team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
		require.NoError(t, err)

		commandResponse, appErr := th.p.executeConnectBotCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "The bot account is already connected to MS Teams. Please disconnect the bot account first before connecting again.")
	})

	t.Run("successfully started connection", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeConnectBotCommand(args)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		ephemeralPost := th.retrieveEphemeralPost(t, args.UserId, args.ChannelId)

		expectedMessage := fmt.Sprintf(`\[Click here to connect the bot account\]\(%s/connect\?isBot&post_id=(.*)&channel_id=%s\)`, th.p.GetURL(), args.ChannelId)
		result, _ := regexp.MatchString(expectedMessage, ephemeralPost.Message)
		assert.True(t, result)
	})
}

func TestGetAutocompleteData(t *testing.T) {
	for _, testCase := range []struct {
		description        string
		syncLinkedChannels bool
		autocompleteData   *model.AutocompleteData
	}{
		{
			description:        "Successfully get all auto complete data",
			syncLinkedChannels: true,
			autocompleteData: &model.AutocompleteData{
				Trigger:   "msteams",
				Hint:      "[command]",
				HelpText:  "Manage MS Teams linked channels",
				RoleID:    model.SystemUserRoleId,
				Arguments: []*model.AutocompleteArg{},
				SubCommands: []*model.AutocompleteData{
					{
						Trigger:  "link",
						Hint:     "[msteams-team-id] [msteams-channel-id]",
						HelpText: "Link current channel to a MS Teams channel",
						RoleID:   model.SystemUserRoleId,
						Arguments: []*model.AutocompleteArg{
							{
								HelpText: "[msteams-team-id]",
								Type:     "DynamicList",
								Required: true,
								Data: &model.AutocompleteDynamicListArg{
									FetchURL: "plugins/com.mattermost.msteams-sync/autocomplete/teams",
								},
							},
							{
								HelpText: "[msteams-channel-id]",
								Type:     "DynamicList",
								Required: true,
								Data: &model.AutocompleteDynamicListArg{
									FetchURL: "plugins/com.mattermost.msteams-sync/autocomplete/channels",
								},
							},
						},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "unlink",
						HelpText:    "Unlink the current channel from the MS Teams channel",
						RoleID:      model.SystemUserRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "show",
						HelpText:    "Show MS Teams linked channel",
						RoleID:      model.SystemUserRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "show-links",
						HelpText:    "Show all MS Teams linked channels",
						RoleID:      model.SystemAdminRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
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
						Trigger:     "connect-bot",
						HelpText:    "Connect the bot account (only system admins can do this)",
						RoleID:      model.SystemAdminRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "disconnect-bot",
						HelpText:    "Disconnect the bot account (only system admins can do this)",
						RoleID:      model.SystemAdminRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:  "promote",
						HelpText: "Promote a user from synthetic user account to regular mattermost account",
						RoleID:   model.SystemAdminRoleId,
						Arguments: []*model.AutocompleteArg{
							{
								HelpText: "Username of the existing mattermost user",
								Type:     "TextInput",
								Required: true,
								Data: &model.AutocompleteTextArg{
									Hint:    "username",
									Pattern: `^[a-z0-9\.\-_:]+$`,
								},
							},
							{
								HelpText: "The new username after the user is promoted",
								Type:     "TextInput",
								Required: true,
								Data: &model.AutocompleteTextArg{
									Hint:    "new username",
									Pattern: `^[a-z0-9\.\-_:]+$`,
								},
							},
						},
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
											HelpText: "Enable notifications.",
										},
										{
											Item:     "off",
											HelpText: "Disable notifications.",
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
		{
			description:        "Successfully get all auto complete data",
			syncLinkedChannels: false,
			autocompleteData: &model.AutocompleteData{
				Trigger:   "msteams",
				Hint:      "[command]",
				HelpText:  "Manage MS Teams linked channels",
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
						Trigger:     "connect-bot",
						HelpText:    "Connect the bot account (only system admins can do this)",
						RoleID:      model.SystemAdminRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:     "disconnect-bot",
						HelpText:    "Disconnect the bot account (only system admins can do this)",
						RoleID:      model.SystemAdminRoleId,
						Arguments:   []*model.AutocompleteArg{},
						SubCommands: []*model.AutocompleteData{},
					},
					{
						Trigger:  "promote",
						HelpText: "Promote a user from synthetic user account to regular mattermost account",
						RoleID:   model.SystemAdminRoleId,
						Arguments: []*model.AutocompleteArg{
							{
								HelpText: "Username of the existing mattermost user",
								Type:     "TextInput",
								Required: true,
								Data: &model.AutocompleteTextArg{
									Hint:    "username",
									Pattern: `^[a-z0-9\.\-_:]+$`,
								},
							},
							{
								HelpText: "The new username after the user is promoted",
								Type:     "TextInput",
								Required: true,
								Data: &model.AutocompleteTextArg{
									Hint:    "new username",
									Pattern: `^[a-z0-9\.\-_:]+$`,
								},
							},
						},
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
											HelpText: "Enable notifications.",
										},
										{
											Item:     "off",
											HelpText: "Disable notifications.",
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
			autocompleteData := getAutocompleteData(testCase.syncLinkedChannels)
			assert.Equal(t, testCase.autocompleteData, autocompleteData)
		})
	}
}

func TestExecutePromoteCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	sysadmin1 := th.SetupSysadmin(t, team)
	user1 := th.SetupUser(t, team)
	user2 := th.SetupUser(t, team)
	user3 := th.SetupUser(t, team)

	th.SetupWebsocketClientForUser(t, user1.Id)
	th.SetupWebsocketClientForUser(t, sysadmin1.Id)

	t.Run("no parameters", func(t *testing.T) {
		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}
		commandResponse, appErr := th.p.executePromoteUserCommand(args, nil)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Invalid promote command, please pass the current username and promoted username as parameters.")
	})

	t.Run("too many parameters", func(t *testing.T) {
		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}
		parameters := []string{"user1", "user2", "user3"}
		commandResponse, appErr := th.p.executePromoteUserCommand(args, parameters)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Invalid promote command, please pass the current username and promoted username as parameters.")
	})

	t.Run("not a system admin", func(t *testing.T) {
		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}
		parameters := []string{"valid-user", "valid-user"}
		commandResponse, appErr := th.p.executePromoteUserCommand(args, parameters)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Unable to execute the command, only system admins have access to execute this command.")
	})

	t.Run("not an existing user", func(t *testing.T) {
		username := model.NewId()

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}
		parameters := []string{username, username}
		commandResponse, appErr := th.p.executePromoteUserCommand(args, parameters)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, fmt.Sprintf("Error: Unable to promote account %s, user not found", username))
	})

	t.Run("not a known ms teams remote user", func(t *testing.T) {
		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}
		parameters := []string{user2.Username, "newUser2"}
		commandResponse, appErr := th.p.executePromoteUserCommand(args, parameters)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, fmt.Sprintf("Error: Unable to promote account %s, it is not a known msteams user account", user2.Username))
	})

	t.Run("not a remote user", func(t *testing.T) {
		err := th.p.store.SetUserInfo(user3.Id, "team_user_id_"+model.NewId(), nil)
		require.NoError(t, err)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}
		parameters := []string{user3.Username, "newUser3"}
		commandResponse, appErr := th.p.executePromoteUserCommand(args, parameters)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, fmt.Sprintf("Error: Unable to promote account %s, it is already a regular account", user3.Username))
	})

	t.Run("new username already exists", func(t *testing.T) {
		remoteUser := th.SetupRemoteUser(t, team)

		err := th.p.store.SetUserInfo(remoteUser.Id, "team_user_id_"+model.NewId(), nil)
		require.NoError(t, err)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}
		parameters := []string{remoteUser.Username, user3.Username}
		commandResponse, appErr := th.p.executePromoteUserCommand(args, parameters)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, "Error: the promoted username already exists, please use a different username.")
	})

	t.Run("successfully promoted to new username, without @ prefix", func(t *testing.T) {
		remoteUser := th.SetupRemoteUser(t, team)
		newUsername := model.NewId()

		err := th.p.store.SetUserInfo(remoteUser.Id, "team_user_id_"+model.NewId(), nil)
		require.NoError(t, err)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}
		parameters := []string{remoteUser.Username, newUsername}
		commandResponse, appErr := th.p.executePromoteUserCommand(args, parameters)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, fmt.Sprintf("Account %s has been promoted and updated the username to %s", remoteUser.Username, newUsername))
	})

	t.Run("successfully promoted to new username, with @ prefix", func(t *testing.T) {
		remoteUser := th.SetupRemoteUser(t, team)
		newUsername := model.NewId()

		err := th.p.store.SetUserInfo(remoteUser.Id, "team_user_id_"+model.NewId(), nil)
		require.NoError(t, err)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}
		parameters := []string{fmt.Sprintf("@%s", remoteUser.Username), fmt.Sprintf("@%s", newUsername)}
		commandResponse, appErr := th.p.executePromoteUserCommand(args, parameters)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, fmt.Sprintf("Account %s has been promoted and updated the username to %s", remoteUser.Username, newUsername))
	})

	t.Run("successfully promoted to same username, without @ prefix", func(t *testing.T) {
		remoteUser := th.SetupRemoteUser(t, team)

		err := th.p.store.SetUserInfo(remoteUser.Id, "team_user_id_"+model.NewId(), nil)
		require.NoError(t, err)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}
		parameters := []string{remoteUser.Username, remoteUser.Username}
		commandResponse, appErr := th.p.executePromoteUserCommand(args, parameters)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, fmt.Sprintf("Account %s has been promoted and updated the username to %s", remoteUser.Username, remoteUser.Username))
	})

	t.Run("successfully promoted to same username, with @ prefix", func(t *testing.T) {
		remoteUser := th.SetupRemoteUser(t, team)

		err := th.p.store.SetUserInfo(remoteUser.Id, "team_user_id_"+model.NewId(), nil)
		require.NoError(t, err)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}
		parameters := []string{fmt.Sprintf("@%s", remoteUser.Username), fmt.Sprintf("@%s", remoteUser.Username)}
		commandResponse, appErr := th.p.executePromoteUserCommand(args, parameters)
		require.Nil(t, appErr)
		assertNoCommandResponse(t, commandResponse)
		assertEphemeralResponse(th, t, args, fmt.Sprintf("Account %s has been promoted and updated the username to %s", remoteUser.Username, remoteUser.Username))
	})
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
					enabled:  model.NewBool(true),
					expected: "Notifications from MSTeams are currently enabled.",
				},
				{
					name:     "disabled",
					enabled:  model.NewBool(false),
					expected: "Notifications from MSTeams are currently disabled.",
				},
				{
					name:     "not set",
					enabled:  nil,
					expected: "Notifications from MSTeams are currently disabled.",
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
			{name: "was enabled", enabled: model.NewBool(true)},
			{name: "was disabled", enabled: model.NewBool(false)},
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
				assertEphemeralResponse(th, t, args, "Notifications from MSTeams are now enabled.")

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
			{name: "was enabled", enabled: model.NewBool(true)},
			{name: "was disabled", enabled: model.NewBool(false)},
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
				assertEphemeralResponse(th, t, args, "Notifications from MSTeams are now disabled.")

				require.False(t, th.p.getNotificationPreference(user1.Id))
			})
		}
	})
}
