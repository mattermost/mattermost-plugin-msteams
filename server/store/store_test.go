package store

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/stretchr/testify/assert"
)

func setupTestStore(api *plugintest.API) (*SQLStore, *plugintest.API) {
	store := &SQLStore{}
	store.api = api
	return store, api
}

func TestGetAvatarCache(t *testing.T) {
	for _, test := range []struct {
		Name                 string
		SetupAPI             func(*plugintest.API)
		ExpectedErrorMessage string
	}{
		{
			Name: "GetAvatarCache: Error while getting the avatar cache",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVGet", avatarKey+testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the avatar cache"))
			},
			ExpectedErrorMessage: "unable to get the avatar cache",
		},
		{
			Name: "GetAvatarCache: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVGet", avatarKey+testutils.GetID()).Return([]byte("mock data"), nil)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			store, api := setupTestStore(&plugintest.API{})
			test.SetupAPI(api)
			resp, err := store.GetAvatarCache(testutils.GetID())

			if test.ExpectedErrorMessage != "" {
				assert.Contains(err.Error(), test.ExpectedErrorMessage)
				assert.Nil(resp)
			} else {
				assert.Nil(err)
				assert.NotNil(resp)
			}
		})
	}
}

func TestSetAvatarCache(t *testing.T) {
	for _, test := range []struct {
		Name                 string
		SetupAPI             func(*plugintest.API)
		ExpectedErrorMessage string
	}{
		{
			Name: "SetAvatarCache: Error while setting the avatar cache",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithExpiry", avatarKey+testutils.GetID(), []byte{10}, int64(avatarCacheTime)).Return(testutils.GetInternalServerAppError("unable to set the avatar cache"))
			},
			ExpectedErrorMessage: "unable to set the avatar cache",
		},
		{
			Name: "SetAvatarCache: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithExpiry", avatarKey+testutils.GetID(), []byte{10}, int64(avatarCacheTime)).Return(nil)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			store, api := setupTestStore(&plugintest.API{})
			test.SetupAPI(api)
			err := store.SetAvatarCache(testutils.GetID(), []byte{10})

			if test.ExpectedErrorMessage != "" {
				assert.Contains(err.Error(), test.ExpectedErrorMessage)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestCheckEnabledTeamByTeamID(t *testing.T) {
	for _, test := range []struct {
		Name           string
		SetupAPI       func(*plugintest.API)
		EnabledTeams   func() []string
		ExpectedResult bool
	}{
		{
			Name:     "CheckEnabledTeamByTeamID: Emmpty enabled team",
			SetupAPI: func(api *plugintest.API) {},
			EnabledTeams: func() []string {
				return []string{""}
			},
			ExpectedResult: true,
		},
		{
			Name: "CheckEnabledTeamByTeamID: Unable to get the team",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetTeam", "mockTeamID").Return(nil, testutils.GetInternalServerAppError("unable to get the team"))
			},
			EnabledTeams: func() []string {
				return []string{"mockTeamsTeam"}
			},
		},
		{
			Name: "CheckEnabledTeamByTeamID: Enabled team does not matches",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetTeam", "mockTeamID").Return(&model.Team{
					Name: "differentTeam",
				}, nil)
			},
			EnabledTeams: func() []string {
				return []string{"mockTeamsTeam"}
			},
		},
		{
			Name: "CheckEnabledTeamByTeamID: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetTeam", "mockTeamID").Return(&model.Team{
					Name: "mockTeamsTeam",
				}, nil)
			},
			EnabledTeams: func() []string {
				return []string{"mockTeamsTeam"}
			},
			ExpectedResult: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			store, api := setupTestStore(&plugintest.API{})
			test.SetupAPI(api)
			store.enabledTeams = test.EnabledTeams
			resp := store.CheckEnabledTeamByTeamID("mockTeamID")

			assert.Equal(test.ExpectedResult, resp)
		})
	}
}
