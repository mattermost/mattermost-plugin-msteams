package main

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func TestGetNotificationStatus(t *testing.T) {
	th := setupTestHelper(t)

	setup := func(t *testing.T) string {
		t.Helper()

		team := th.SetupTeam(t)
		user := th.SetupUser(t, team)
		err := th.p.API.DeletePreferencesForUser(user.Id, []model.Preference{
			{UserId: user.Id, Category: PreferenceCategoryPlugin, Name: storemodels.PreferenceNameNotification},
		})
		require.Nil(t, err)

		return user.Id
	}

	t.Run("user without the preference should return false", func(t *testing.T) {
		assert := require.New(t)
		userID := setup(t)

		assert.False(th.p.getNotificationPreference(userID))
	})

	t.Run("user with the preference set to off should return false", func(t *testing.T) {
		assert := require.New(t)
		userID := setup(t)

		err := th.p.API.UpdatePreferencesForUser(userID, []model.Preference{
			{
				UserId:   userID,
				Category: PreferenceCategoryPlugin,
				Name:     storemodels.PreferenceNameNotification,
				Value:    storemodels.PreferenceValueNotificationOff,
			},
		})
		assert.Nil(err)

		assert.False(th.p.getNotificationPreference(userID))
	})

	t.Run("user with the preference set to on should return true", func(t *testing.T) {
		assert := require.New(t)
		userID := setup(t)

		err := th.p.API.UpdatePreferencesForUser(userID, []model.Preference{
			{
				UserId:   userID,
				Category: PreferenceCategoryPlugin,
				Name:     storemodels.PreferenceNameNotification,
				Value:    storemodels.PreferenceValueNotificationOn,
			},
		})
		assert.Nil(err)

		assert.True(th.p.getNotificationPreference(userID))
	})
}

func TestSetNotificationStatus(t *testing.T) {
	th := setupTestHelper(t)

	setup := func(t *testing.T) string {
		t.Helper()

		team := th.SetupTeam(t)
		user := th.SetupUser(t, team)
		return user.Id
	}

	t.Run("set to true should update the preference to on", func(t *testing.T) {
		assert := require.New(t)
		userID := setup(t)

		err := th.p.setNotificationPreference(userID, true)
		assert.Nil(err)

		pref, err := th.p.API.GetPreferenceForUser(userID, PreferenceCategoryPlugin, storemodels.PreferenceNameNotification)
		assert.Nil(err)
		assert.Equal(storemodels.PreferenceValueNotificationOn, pref.Value)
	})

	t.Run("set to false should update the preference to off", func(t *testing.T) {
		assert := require.New(t)
		userID := setup(t)

		err := th.p.setNotificationPreference(userID, false)
		assert.Nil(err)

		pref, err := th.p.API.GetPreferenceForUser(userID, PreferenceCategoryPlugin, storemodels.PreferenceNameNotification)
		assert.Nil(err)
		assert.Equal(storemodels.PreferenceValueNotificationOff, pref.Value)
	})
}
