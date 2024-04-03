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

func TestUpdateAutomutingOnUserConnect(t *testing.T) {
	th := setupTestHelper(t)

	setup := func(t *testing.T) (*Plugin, *model.User, *model.Channel) {
		t.Helper()
		th.Reset(t)

		team := th.SetupTeam(t)

		user := th.SetupUser(t, team)

		channel := th.SetupPublicChannel(t, team, WithMembers(user))
		th.LinkChannel(t, team, channel, user)

		assertChannelNotAutomuted(t, th.p, channel.Id, user.Id)

		return th.p, user, channel
	}

	t.Run("should do nothing when a user connects without their primary platform set", func(t *testing.T) {
		p, user, channel := setup(t)

		automuteEnabled, err := p.updateAutomutingOnUserConnect(user.Id)
		require.NoError(t, err)

		assert.Equal(t, false, automuteEnabled)

		pref, _ := p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "", pref.Value)

		assertChannelNotAutomuted(t, p, channel.Id, user.Id)
	})

	t.Run("should do nothing when a user connects with their primary platform set to MM", func(t *testing.T) {
		p, user, channel := setup(t)

		appErr := p.API.UpdatePreferencesForUser(user.Id, []model.Preference{{
			UserId:   user.Id,
			Category: PreferenceCategoryPlugin,
			Name:     PreferenceNamePlatform,
			Value:    PreferenceValuePlatformMM,
		}})
		require.Nil(t, appErr)

		automuteEnabled, err := p.updateAutomutingOnUserConnect(user.Id)
		require.NoError(t, err)

		assert.Equal(t, false, automuteEnabled)

		pref, _ := p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "", pref.Value)

		assertChannelNotAutomuted(t, p, channel.Id, user.Id)
	})

	t.Run("should enable automute when a user connects with their primary platform set to Teams", func(t *testing.T) {
		p, user, channel := setup(t)

		appErr := p.API.UpdatePreferencesForUser(user.Id, []model.Preference{{
			UserId:   user.Id,
			Category: PreferenceCategoryPlugin,
			Name:     PreferenceNamePlatform,
			Value:    PreferenceValuePlatformMSTeams,
		}})
		require.Nil(t, appErr)

		automuteEnabled, err := p.updateAutomutingOnUserConnect(user.Id)
		require.NoError(t, err)

		assert.Equal(t, true, automuteEnabled)

		pref, _ := p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "true", pref.Value)

		assertChannelAutomuted(t, p, channel.Id, user.Id)
	})

	t.Run("should do nothing if somehow called twice in a row", func(t *testing.T) {
		p, user, channel := setup(t)

		appErr := p.API.UpdatePreferencesForUser(user.Id, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNameAutomuteEnabled,
				Value:    "true",
			},
		})
		require.Nil(t, appErr)

		automuteEnabled, err := p.updateAutomutingOnUserConnect(user.Id)
		require.NoError(t, err)

		assert.Equal(t, false, automuteEnabled)

		pref, _ := p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		assert.Equal(t, "true", pref.Value)

		assertChannelNotAutomuted(t, p, channel.Id, user.Id)
	})
}

func TestUpdateAutomutingOnUserDisconnect(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	setup := func(t *testing.T) (*Plugin, *model.User, *model.Channel) {
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

		return th.p, user, linkedChannel
	}

	t.Run("should disable automute when a user disconnects who previously had automuting enabled", func(t *testing.T) {
		p, user, channel := setup(t)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		// Confirm that the user starts with automute enabled and a muted channel
		pref, _ := p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		require.Equal(t, "true", pref.Value)

		assertChannelAutomuted(t, p, channel.Id, user.Id)

		// Disconnect
		automuteDisabled, err := p.updateAutomutingOnUserDisconnect(user.Id)

		require.NoError(t, err)
		assert.Equal(t, true, automuteDisabled)

		// The user should no longer have automute enabled and the channel should no longer be muted
		pref, _ = p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		require.Equal(t, "false", pref.Value)

		assertChannelNotAutomuted(t, p, channel.Id, user.Id)
	})

	t.Run("should do nothing when a user disconnects without automuting enabled", func(t *testing.T) {
		p, user, channel := setup(t)

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMM,
			},
		})

		// Confirm that the user starts with automute enabled and a muted channel
		pref, _ := p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		require.Equal(t, "", pref.Value)

		assertChannelNotAutomuted(t, p, channel.Id, user.Id)

		// Disconnect
		automuteDisabled, err := p.updateAutomutingOnUserDisconnect(user.Id)

		require.NoError(t, err)
		assert.Equal(t, false, automuteDisabled)

		// Confirm nothing changed
		pref, _ = p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		require.Equal(t, "", pref.Value)

		assertChannelNotAutomuted(t, p, channel.Id, user.Id)
	})

	t.Run("should not unmute a manually muted unlinked channel when a user disconnects", func(t *testing.T) {
		p, user, _ := setup(t)

		unlinkedChannel := th.SetupPublicChannel(t, team, WithMembers(user))

		p.PreferencesHaveChanged(&plugin.Context{}, []model.Preference{
			{
				UserId:   user.Id,
				Category: PreferenceCategoryPlugin,
				Name:     PreferenceNamePlatform,
				Value:    PreferenceValuePlatformMSTeams,
			},
		})

		// Confirm that the user starts with automute enabled and the channel is not muted
		pref, _ := p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		require.Equal(t, "true", pref.Value)

		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)

		// Disconnect
		automuteDisabled, err := p.updateAutomutingOnUserDisconnect(user.Id)

		require.NoError(t, err)
		assert.Equal(t, true, automuteDisabled)

		// The user should no longer have automute enabled and the channel is still not muted
		pref, _ = p.API.GetPreferenceForUser(user.Id, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
		require.Equal(t, "false", pref.Value)

		assertChannelNotAutomuted(t, p, unlinkedChannel.Id, user.Id)
	})
}
