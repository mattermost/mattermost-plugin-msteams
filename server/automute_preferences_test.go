package main

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestUpdateAutomutingOnPreferencesChanged(t *testing.T) {
	th := setupTestHelper(t)

	team := th.SetupTeam(t)

	setup := func(t *testing.T) (*Plugin, *model.User, *model.Channel, *model.Channel, *model.Channel) {
		t.Helper()
		th.Reset(t)

		user := th.SetupUser(t, team)
		err := th.p.store.SetUserInfo(user.Id, "team_user_id", &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(10 * time.Minute)})
		require.NoError(t, err)

		linkedChannel := th.SetupPublicChannel(t, team, WithMembers(user))

		channelLink := storemodels.ChannelLink{
			MattermostTeamID:    team.Id,
			MattermostChannelID: linkedChannel.Id,
			MSTeamsTeam:         model.NewId(),
			MSTeamsChannel:      model.NewId(),
			Creator:             user.Id,
		}
		err = th.p.store.StoreChannelLink(&channelLink)
		require.NoError(t, err)

		unlinkedChannel := th.SetupPublicChannel(t, team, WithMembers(user))

		otherUser := th.SetupUser(t, team)
		dmChannel, appErr := th.p.API.GetDirectChannel(user.Id, otherUser.Id)
		require.Nil(t, appErr)

		assertChannelNotAutomuted(t, th.p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, th.p, unlinkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, th.p, dmChannel.Id, user.Id)

		return th.p, user, linkedChannel, unlinkedChannel, dmChannel
	}

	t.Run("should mute linked channels when their primary platform changes from MM to MS Teams", func(t *testing.T) {
		p, user, linkedChannel, unlinkedChannel, dmChannel := setup(t)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMM,
			},
		})

		assertUserHasAutomuteDisabled(t, p, user.Id)

		assertChannelNotAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, dmChannel.Id, user.Id)
		th.assertDMFromUser(t, p.userID, user.Id, userChoseMattermostPrimaryMessage)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		assertUserHasAutomuteEnabled(t, p, user.Id)

		assertChannelAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)
		assertChannelAutomuted(t, p, dmChannel.Id, user.Id)
		th.assertDMFromUser(t, p.userID, user.Id, userChoseTeamsPrimaryMessage)
	})

	t.Run("should unmute linked channels when their primary platform changes from MS Teams to MM", func(t *testing.T) {
		p, user, linkedChannel, unlinkedChannel, dmChannel := setup(t)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		assertUserHasAutomuteEnabled(t, p, user.Id)

		assertChannelAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)
		assertChannelAutomuted(t, p, dmChannel.Id, user.Id)
		th.assertDMFromUser(t, p.userID, user.Id, userChoseTeamsPrimaryMessage)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMM,
			},
		})

		assertUserHasAutomuteDisabled(t, p, user.Id)

		assertChannelNotAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, dmChannel.Id, user.Id)
		th.assertDMFromUser(t, p.userID, user.Id, userChoseMattermostPrimaryMessage)
	})

	t.Run("should unmute linked channels when a MS Teams user disconnects", func(t *testing.T) {
		p, user, linkedChannel, unlinkedChannel, dmChannel := setup(t)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		assertUserHasAutomuteEnabled(t, p, user.Id)

		assertChannelAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)
		assertChannelAutomuted(t, p, dmChannel.Id, user.Id)
		th.assertDMFromUser(t, p.userID, user.Id, userChoseTeamsPrimaryMessage)

		checkTime := model.GetMillisForTime(time.Now())
		args := &model.CommandArgs{
			UserId: user.Id,
		}
		_, err := th.p.executeDisconnectCommand(args)
		require.NoError(t, err)

		assertChannelNotAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, dmChannel.Id, user.Id)
		th.assertNoDMFromUser(t, p.userID, user.Id, checkTime)
	})

	t.Run("should do nothing when unrelated preferences change", func(t *testing.T) {
		p, user, _, _, _ := setup(t)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: model.PreferenceCategoryDisplaySettings,
				Name:     model.PreferenceNameChannelDisplayMode,
				Value:    "full",
			},
		})
		th.assertNoDMFromUser(t, p.userID, user.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
	})

	t.Run("should do nothing when an unconnected user turns on automuting", func(t *testing.T) {
		p, _, linkedChannel, unlinkedChannel, _ := setup(t)

		unconnectedUser := th.SetupUser(t, team)
		otherUser := th.SetupUser(t, team)

		_, appErr := p.API.AddUserToChannel(linkedChannel.Id, unconnectedUser.Id, "")
		require.Nil(t, appErr)
		_, appErr = p.API.AddUserToChannel(unlinkedChannel.Id, unconnectedUser.Id, "")
		require.Nil(t, appErr)

		dmChannel, appErr := p.API.GetDirectChannel(unconnectedUser.Id, otherUser.Id)
		require.Nil(t, appErr)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   unconnectedUser.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		assertUserHasAutomuteDisabled(t, p, unconnectedUser.Id)

		assertChannelNotAutomuted(t, p, linkedChannel.Id, unconnectedUser.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, unconnectedUser.Id)
		assertChannelNotAutomuted(t, p, dmChannel.Id, unconnectedUser.Id)
		th.assertNoDMFromUser(t, p.userID, unconnectedUser.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
	})

	t.Run("should not affect other users when a connected user turns on automuting", func(t *testing.T) {
		p, user, linkedChannel, unlinkedChannel, _ := setup(t)

		connectedUser := th.SetupUser(t, team)
		th.ConnectUser(t, connectedUser.Id)

		_, appErr := p.API.AddUserToChannel(linkedChannel.Id, connectedUser.Id, "")
		require.Nil(t, appErr)
		_, appErr = p.API.AddUserToChannel(unlinkedChannel.Id, connectedUser.Id, "")
		require.Nil(t, appErr)

		unconnectedUser := th.SetupUser(t, team)

		_, appErr = p.API.AddUserToChannel(linkedChannel.Id, unconnectedUser.Id, "")
		require.Nil(t, appErr)
		_, appErr = p.API.AddUserToChannel(unlinkedChannel.Id, unconnectedUser.Id, "")
		require.Nil(t, appErr)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		assertUserHasAutomuteEnabled(t, p, user.Id)
		th.assertDMFromUser(t, p.userID, user.Id, userChoseTeamsPrimaryMessage)

		assertChannelAutomuted(t, p, linkedChannel.Id, user.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)

		assertUserHasAutomuteDisabled(t, p, connectedUser.Id)

		assertChannelNotAutomuted(t, p, linkedChannel.Id, connectedUser.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, connectedUser.Id)

		assertUserHasAutomuteDisabled(t, p, unconnectedUser.Id)

		assertChannelNotAutomuted(t, p, linkedChannel.Id, unconnectedUser.Id)
		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, unconnectedUser.Id)
	})

	t.Run("should be able to mute a lot of channels at once", func(t *testing.T) {
		p, user, _, _, _ := setup(t)

		numChannels := 1000
		channels := make([]*model.Channel, numChannels)
		for i := 0; i < numChannels; i++ {
			channel := th.SetupPublicChannel(t, team, WithMembers(user))

			th.LinkChannel(t, team, channel, user)

			channels[i] = channel
		}

		for _, channel := range channels {
			assertChannelNotAutomuted(t, p, channel.Id, user.Id)
		}

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		assertUserHasAutomuteEnabled(t, p, user.Id)
		th.assertDMFromUser(t, p.userID, user.Id, userChoseTeamsPrimaryMessage)

		for _, channel := range channels {
			assertChannelAutomuted(t, p, channel.Id, user.Id)
		}

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMM,
			},
		})

		assertUserHasAutomuteDisabled(t, p, user.Id)
		th.assertDMFromUser(t, p.userID, user.Id, userChoseMattermostPrimaryMessage)

		for _, channel := range channels {
			assertChannelNotAutomuted(t, p, channel.Id, user.Id)
		}
	})
}

func TestGetUsersWhoChangedPlatform(t *testing.T) {
	preferences := []model.Preference{
		{
			UserId:   "user1",
			Category: PreferenceCategoryPlugin,
			Name:     PreferenceNamePlatform,
			Value:    PreferenceValuePlatformMM,
		},
		{
			UserId:   "user2",
			Category: PreferenceCategoryPlugin,
			Name:     PreferenceNamePlatform,
			Value:    PreferenceValuePlatformMSTeams,
		},
		{
			UserId:   "user3",
			Category: model.PreferenceCategoryDisplaySettings,
			Name:     model.PreferenceNameChannelDisplayMode,
			Value:    "full",
		},
	}

	usersWithTeamsPrimary, usersWithMMPrimary := getUsersWhoChangedPlatform(preferences)

	assert.Equal(t, []string{"user2"}, usersWithTeamsPrimary)
	assert.Equal(t, []string{"user1"}, usersWithMMPrimary)
}
