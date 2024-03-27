package main

import (
	"encoding/json"
	"fmt"
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
	require.NotNil(t, actual)
	require.Equal(t, &model.CommandResponse{}, actual)
}

func assertCommandResponse(t *testing.T, text string, actual *model.CommandResponse) {
	require.NotNil(t, actual)
	require.Equal(t, &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         text,
	}, actual)
}

func assertEphemeralMessage(t *testing.T, websocketClient *model.WebSocketClient, channelID, message string) {
	for {
		select {
		case event := <-websocketClient.EventChannel:
			if event.EventType() == model.WebsocketEventEphemeralMessage {
				data := event.GetData()
				t.Logf("%+v\n", data)
				postJSON, ok := data["post"].(string)
				require.True(t, ok, "failed to find post in ephemeral message websocket event")

				var post model.Post
				err := json.Unmarshal([]byte(postJSON), &post)
				require.NoError(t, err)

				assert.Equal(t, channelID, post.ChannelId)
				assert.Equal(t, message, post.Message)

				// If we get this far, we're good!
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("failed to get websocket event for ephemeral message")
		}
	}
}

func TestExecuteUnlinkCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	user2 := th.SetupUser(t, team)
	user3 := th.SetupUser(t, team)

	t.Run("invalid channel", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = true
		commandResponse, appErr := th.p.executeUnlinkCommand(&model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		})
		require.Nil(t, appErr)
		assertCommandResponse(t, "Unable to get the current channel information.", commandResponse)
	})

	t.Run("channel is a dm", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = true
		channel, err := th.p.API.GetDirectChannel(user1.Id, user2.Id)
		require.Nil(t, err)

		commandResponse, appErr := th.p.executeUnlinkCommand(&model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		})
		require.Nil(t, appErr)
		assertCommandResponse(t, "Linking/unlinking a direct or group message is not allowed", commandResponse)
	})

	t.Run("channel is a gm", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = true
		channel, err := th.p.API.GetGroupChannel([]string{user1.Id, user2.Id, user3.Id})
		require.Nil(t, err)

		commandResponse, appErr := th.p.executeUnlinkCommand(&model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		})
		require.Nil(t, appErr)
		assertCommandResponse(t, "Linking/unlinking a direct or group message is not allowed", commandResponse)
	})

	t.Run("not a channel admin", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = true
		channel := th.SetupPublicChannel(t, team)

		commandResponse, appErr := th.p.executeUnlinkCommand(&model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		})
		require.Nil(t, appErr)
		assertCommandResponse(t, "Unable to unlink the channel, you have to be a channel admin to unlink it.", commandResponse)
	})

	t.Run("not a linked channel", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = true
		channel := th.SetupPublicChannel(t, team)
		_, appErr := th.p.API.AddUserToChannel(channel.Id, user1.Id, user1.Id)
		require.Nil(t, appErr)
		_, appErr = th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		commandResponse, appErr := th.p.executeUnlinkCommand(&model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		})
		require.Nil(t, appErr)
		assertCommandResponse(t, "This Mattermost channel is not linked to any MS Teams channel.", commandResponse)
	})

	t.Run("successfully unlinked", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = true
		channel := th.SetupPublicChannel(t, team)
		_, appErr := th.p.API.AddUserToChannel(channel.Id, user1.Id, user1.Id)
		require.Nil(t, appErr)
		_, appErr = th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
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

		commandResponse, appErr := th.p.executeUnlinkCommand(&model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		})
		require.Nil(t, appErr)
		assertCommandResponse(t, "The MS Teams channel is no longer linked to this Mattermost channel.", commandResponse)
	})

	t.Run("successfully unlinked, sync msg enabled", func(t *testing.T) {
		th.p.configuration.DisableSyncMsg = false
		channel := th.SetupPublicChannel(t, team)
		_, appErr := th.p.API.AddUserToChannel(channel.Id, user1.Id, user1.Id)
		require.Nil(t, appErr)
		_, appErr = th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
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

		commandResponse, appErr := th.p.executeUnlinkCommand(&model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		})
		require.Nil(t, appErr)
		assertCommandResponse(t, "The MS Teams channel is no longer linked to this Mattermost channel.", commandResponse)
	})
}

func TestExecuteShowCommand(t *testing.T) {
	th := setupTestHelper(t)

	t.Run("invalid channel id", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			ChannelId: "",
		}

		commandResponse, appErr := th.p.executeShowCommand(args)
		require.Nil(t, appErr)
		assertCommandResponse(t, "Link doesn't exist.", commandResponse)
	})

	t.Run("unlinked channel", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			ChannelId: "unlinked",
		}

		commandResponse, appErr := th.p.executeShowCommand(args)
		require.Nil(t, appErr)
		assertCommandResponse(t, "Link doesn't exist.", commandResponse)
	})

	t.Run("unable to get the MS Teams team", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
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
		assertCommandResponse(t, "Unable to get the MS Teams team information.", commandResponse)
	})

	t.Run("unable to get the MS Teams channel", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
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
		assertCommandResponse(t, "Unable to get the MS Teams channel information.", commandResponse)
	})

	t.Run("successfully executed show command", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
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
		assertCommandResponse(t, "This channel is linked to the MS Teams Channel \"Channel\" in the Team \"Team\".", commandResponse)
	})
}

func TestExecuteShowLinksCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	channel1 := th.SetupPublicChannel(t, team)
	user1 := th.SetupUser(t, team)
	sysadmin1 := th.SetupSysadmin(t, team)

	t.Run("not a system admin", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeShowLinksCommand(args)
		require.Nil(t, appErr)
		assertCommandResponse(t, "Unable to execute the command, only system admins have access to execute this command.", commandResponse)
	})

	t.Run("no links", func(t *testing.T) {
		th.Reset(t)

		client := th.SetupClient(t, sysadmin1)
		websocketClient := th.SetupWebsocketClient(t, client)
		websocketClient.Listen()
		defer websocketClient.Close()

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: "unlinked",
		}

		commandResponse, appErr := th.p.executeShowLinksCommand(args)
		require.Nil(t, appErr)
		assertCommandResponse(t, "No links present.", commandResponse)
	})

	t.Run("failed fetching teams info", func(t *testing.T) {
		th.Reset(t)

		client := th.SetupClient(t, sysadmin1)
		websocketClient := th.SetupWebsocketClient(t, client)
		websocketClient.Listen()
		defer websocketClient.Close()

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

		assertEphemeralMessage(t, websocketClient, "unlinked", "Please wait while your request is being processed.")
		assertEphemeralMessage(t, websocketClient, "unlinked", fmt.Sprintf(`| Mattermost Team | Mattermost Channel | MS Teams Team | MS Teams Channel |
| :------|:--------|:-------|:-----------|
|%s|%s||Teams Channel|
There were some errors while fetching information. Please check the server logs.`, team.DisplayName, channel1.DisplayName))
	})

	t.Run("one link", func(t *testing.T) {
		th.Reset(t)

		client := th.SetupClient(t, sysadmin1)
		websocketClient := th.SetupWebsocketClient(t, client)
		websocketClient.Listen()
		defer websocketClient.Close()

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

		assertEphemeralMessage(t, websocketClient, "unlinked", "Please wait while your request is being processed.")
		assertEphemeralMessage(t, websocketClient, "unlinked", fmt.Sprintf(`| Mattermost Team | Mattermost Channel | MS Teams Team | MS Teams Channel |
| :------|:--------|:-------|:-----------|
|%s|%s|Teams Team|Teams Channel|`, team.DisplayName, channel1.DisplayName))
	})
}

func TestExecuteDisconnectCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)

	t.Run("not connected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeDisconnectCommand(args)
		require.Nil(t, appErr)
		assertCommandResponse(t, "Error: the account is not connected", commandResponse)
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
		assertCommandResponse(t, "Error: the account is not connected", commandResponse)
	})

	t.Run("successfully disconnected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		err := th.p.store.SetUserInfo(user1.Id, "team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
		require.NoError(t, err)

		commandResponse, appErr := th.p.executeDisconnectCommand(args)
		require.Nil(t, appErr)
		assertCommandResponse(t, "Your account has been disconnected.", commandResponse)
	})
}

func TestExecuteDisconnectBotCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	sysadmin1 := th.SetupSysadmin(t, team)

	t.Run("not a system admin", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeDisconnectBotCommand(args)
		require.Nil(t, appErr)
		assertCommandResponse(t, "Unable to disconnect the bot account, only system admins can disconnect the bot account.", commandResponse)
	})

	t.Run("bot user is not connected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeDisconnectBotCommand(args)
		require.Nil(t, appErr)
		assertCommandResponse(t, "Error: unable to find the connected bot account", commandResponse)
	})

	t.Run("successfully disconnected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}

		err := th.p.store.SetUserInfo(th.p.userID, "bot_team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})

		require.NoError(t, err)

		commandResponse, appErr := th.p.executeDisconnectBotCommand(args)
		require.Nil(t, appErr)
		assertCommandResponse(t, "The bot account has been disconnected.", commandResponse)
	})
}

func TestExecuteLinkCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	user2 := th.SetupUser(t, team)
	user3 := th.SetupUser(t, team)

	t.Run("unknown channel", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{})
		require.Nil(t, appErr)
		assertCommandResponse(t, "Unable to get the current channel information.", commandResponse)
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
		assertCommandResponse(t, "Linking/unlinking a direct or group message is not allowed", commandResponse)
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
		assertCommandResponse(t, "Linking/unlinking a direct or group message is not allowed", commandResponse)
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
		assertCommandResponse(t, "Unable to link the channel. You have to be a channel admin to link it.", commandResponse)
	})

	t.Run("missing parameters", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team)
		_, appErr := th.p.API.AddUserToChannel(channel.Id, user1.Id, user1.Id)
		require.Nil(t, appErr)
		_, appErr = th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{})
		require.Nil(t, appErr)
		assertCommandResponse(t, "Invalid link command, please pass the MS Teams team id and channel id as parameters.", commandResponse)
	})

	t.Run("linking channel not allowed in this team", func(t *testing.T) {
		t.Skip("not yet implemented")
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team)
		_, appErr := th.p.API.AddUserToChannel(channel.Id, user1.Id, user1.Id)
		require.Nil(t, appErr)
		_, appErr = th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{"msteams_team_id", "msteams_channel_id"})
		require.Nil(t, appErr)
		assertCommandResponse(t, "This team is not enabled for MS Teams sync.", commandResponse)
	})

	t.Run("already linked", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team)
		_, appErr := th.p.API.AddUserToChannel(channel.Id, user1.Id, user1.Id)
		require.Nil(t, appErr)
		_, appErr = th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
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
		assertCommandResponse(t, "A link for this channel already exists. Please unlink the channel before you link again with another channel.", commandResponse)
	})

	t.Run("another channel already linked", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team)
		_, appErr := th.p.API.AddUserToChannel(channel.Id, user1.Id, user1.Id)
		require.Nil(t, appErr)
		_, appErr = th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
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
		assertCommandResponse(t, "A link for this channel already exists. Please unlink the channel before you link again with another channel.", commandResponse)
	})

	t.Run("user not connected", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team)
		_, appErr := th.p.API.AddUserToChannel(channel.Id, user1.Id, user1.Id)
		require.Nil(t, appErr)
		_, appErr = th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: channel.Id,
		}

		commandResponse, appErr := th.p.executeLinkCommand(args, []string{"msteams_team_id", "msteams_channel_id"})
		require.Nil(t, appErr)
		assertCommandResponse(t, "Unable to link the channel, looks like your account is not connected to MS Teams", commandResponse)
	})

	t.Run("not found on Teams", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team)
		_, appErr := th.p.API.AddUserToChannel(channel.Id, user1.Id, user1.Id)
		require.Nil(t, appErr)
		_, appErr = th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
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
		assertCommandResponse(t, "MS Teams channel not found or you don't have the permissions to access it.", commandResponse)
	})

	t.Run("failed to subscribe", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team)
		_, appErr := th.p.API.AddUserToChannel(channel.Id, user1.Id, user1.Id)
		require.Nil(t, appErr)
		_, appErr = th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
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
		assertCommandResponse(t, "Unable to subscribe to the channel", commandResponse)
	})

	t.Run("successfully linked", func(t *testing.T) {
		th.Reset(t)

		channel := th.SetupPublicChannel(t, team)
		_, appErr := th.p.API.AddUserToChannel(channel.Id, user1.Id, user1.Id)
		require.Nil(t, appErr)
		_, appErr = th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
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
		assertCommandResponse(t, "The MS Teams channel is now linked to this Mattermost channel.", commandResponse)
	})
}

func TestExecuteConnectCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)

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
		assertCommandResponse(t, "You are already connected to MS Teams. Please disconnect your account first before connecting again.", commandResponse)
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

		assertCommandResponse(t, "You cannot connect your account because the maximum limit of users allowed to connect has been reached. Please contact your system administrator.", commandResponse)
	})

	t.Run("successfully started connection", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeConnectCommand(args)
		require.Nil(t, appErr)

		assertCommandResponse(t, fmt.Sprintf("[Click here to connect your account](%s/connect)", th.p.GetURL()), commandResponse)
	})
}

func TestExecuteConnectBotCommand(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	sysadmin1 := th.SetupSysadmin(t, team)

	t.Run("not a system admin", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeConnectBotCommand(args)
		require.Nil(t, appErr)
		assertCommandResponse(t, "Unable to connect the bot account, only system admins can connect the bot account.", commandResponse)
	})

	t.Run("bot user already connected", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}

		err := th.p.store.SetUserInfo(th.p.userID, "bot_team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
		require.NoError(t, err)

		commandResponse, appErr := th.p.executeConnectBotCommand(args)
		require.Nil(t, appErr)
		assertCommandResponse(t, "The bot account is already connected to MS Teams. Please disconnect the bot account first before connecting again.", commandResponse)
	})

	t.Run("not in whitelist, already at limit", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
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

		commandResponse, appErr := th.p.executeConnectBotCommand(args)
		require.Nil(t, appErr)

		assertCommandResponse(t, "You cannot connect the bot account because the maximum limit of users allowed to connect has been reached.", commandResponse)
	})

	t.Run("successfully started connection", func(t *testing.T) {
		th.Reset(t)

		args := &model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}

		commandResponse, appErr := th.p.executeConnectBotCommand(args)
		require.Nil(t, appErr)
		assertCommandResponse(t, fmt.Sprintf("[Click here to connect the bot account](%s/connect)", th.p.GetURL()), commandResponse)
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

	t.Run("no parameters", func(t *testing.T) {
		commandResponse, appErr := th.p.executePromoteUserCommand(&model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}, nil)
		require.Nil(t, appErr)
		assertCommandResponse(t, "Invalid promote command, please pass the current username and promoted username as parameters.", commandResponse)
	})

	t.Run("too many parameters", func(t *testing.T) {
		commandResponse, appErr := th.p.executePromoteUserCommand(&model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}, []string{"user1", "user2", "user3"})
		require.Nil(t, appErr)
		assertCommandResponse(t, "Invalid promote command, please pass the current username and promoted username as parameters.", commandResponse)
	})

	t.Run("not a system admin", func(t *testing.T) {
		commandResponse, appErr := th.p.executePromoteUserCommand(&model.CommandArgs{
			UserId:    user1.Id,
			ChannelId: model.NewId(),
		}, []string{"valid-user", "valid-user"})
		require.Nil(t, appErr)
		assertCommandResponse(t, "Unable to execute the command, only system admins have access to execute this command.", commandResponse)
	})

	t.Run("not an existing user", func(t *testing.T) {
		username := model.NewId()

		commandResponse, appErr := th.p.executePromoteUserCommand(&model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}, []string{username, username})
		require.Nil(t, appErr)
		assertCommandResponse(t, fmt.Sprintf("Error: Unable to promote account %s, user not found", username), commandResponse)
	})

	t.Run("not a known ms teams remote user", func(t *testing.T) {
		commandResponse, appErr := th.p.executePromoteUserCommand(&model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}, []string{user2.Username, "newUser2"})
		require.Nil(t, appErr)
		assertCommandResponse(t, fmt.Sprintf("Error: Unable to promote account %s, it is not a known msteams user account", user2.Username), commandResponse)
	})

	t.Run("not a remote user", func(t *testing.T) {
		err := th.p.store.SetUserInfo(user3.Id, "team_user_id", nil)
		require.NoError(t, err)

		commandResponse, appErr := th.p.executePromoteUserCommand(&model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}, []string{user3.Username, "newUser3"})
		require.Nil(t, appErr)
		assertCommandResponse(t, fmt.Sprintf("Error: Unable to promote account %s, it is already a regular account", user3.Username), commandResponse)
	})

	t.Run("new username already exists", func(t *testing.T) {
		remoteUser := th.SetupRemoteUser(t, team)

		err := th.p.store.SetUserInfo(remoteUser.Id, "team_user_id", nil)
		require.NoError(t, err)

		commandResponse, appErr := th.p.executePromoteUserCommand(&model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}, []string{remoteUser.Username, user3.Username})
		require.Nil(t, appErr)
		assertCommandResponse(t, "Error: the promoted username already exists, please use a different username.", commandResponse)
	})

	t.Run("successfully promoted to new username, without @ prefix", func(t *testing.T) {
		remoteUser := th.SetupRemoteUser(t, team)
		newUsername := model.NewId()

		err := th.p.store.SetUserInfo(remoteUser.Id, "team_user_id", nil)
		require.NoError(t, err)

		commandResponse, appErr := th.p.executePromoteUserCommand(&model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}, []string{remoteUser.Username, newUsername})
		require.Nil(t, appErr)
		assertCommandResponse(t, fmt.Sprintf("Account %s has been promoted and updated the username to %s", remoteUser.Username, newUsername), commandResponse)
	})

	t.Run("successfully promoted to new username, with @ prefix", func(t *testing.T) {
		remoteUser := th.SetupRemoteUser(t, team)
		newUsername := model.NewId()

		err := th.p.store.SetUserInfo(remoteUser.Id, "team_user_id", nil)
		require.NoError(t, err)

		commandResponse, appErr := th.p.executePromoteUserCommand(&model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}, []string{fmt.Sprintf("@%s", remoteUser.Username), fmt.Sprintf("@%s", newUsername)})
		require.Nil(t, appErr)
		assertCommandResponse(t, fmt.Sprintf("Account %s has been promoted and updated the username to %s", remoteUser.Username, newUsername), commandResponse)
	})

	t.Run("successfully promoted to same username, without @ prefix", func(t *testing.T) {
		remoteUser := th.SetupRemoteUser(t, team)

		err := th.p.store.SetUserInfo(remoteUser.Id, "team_user_id", nil)
		require.NoError(t, err)

		commandResponse, appErr := th.p.executePromoteUserCommand(&model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}, []string{remoteUser.Username, remoteUser.Username})
		require.Nil(t, appErr)
		assertCommandResponse(t, fmt.Sprintf("Account %s has been promoted and updated the username to %s", remoteUser.Username, remoteUser.Username), commandResponse)
	})

	t.Run("successfully promoted to same username, with @ prefix", func(t *testing.T) {
		remoteUser := th.SetupRemoteUser(t, team)

		err := th.p.store.SetUserInfo(remoteUser.Id, "team_user_id", nil)
		require.NoError(t, err)

		commandResponse, appErr := th.p.executePromoteUserCommand(&model.CommandArgs{
			UserId:    sysadmin1.Id,
			ChannelId: model.NewId(),
		}, []string{fmt.Sprintf("@%s", remoteUser.Username), fmt.Sprintf("@%s", remoteUser.Username)})
		require.Nil(t, appErr)
		assertCommandResponse(t, fmt.Sprintf("Account %s has been promoted and updated the username to %s", remoteUser.Username, remoteUser.Username), commandResponse)
	})
}
