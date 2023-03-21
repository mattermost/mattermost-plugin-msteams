package store

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/stretchr/testify/assert"
)

func TestGetAvatarCache(t *testing.T) {
	ttcases := []struct {
		Name                 string
		SetupAPI             func(*plugintest.API) *plugintest.API
		ExpectedErrorMessage string
	}{
		{
			"GetAvatarCache: Error while getting the avatar cache",
			func(a *plugintest.API) *plugintest.API {
				a.On("KVGet", avatarKey+testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the avatar cache"))
				return a
			},
			"unable to get the avatar cache",
		},
		{
			"GetAvatarCache: Valid",
			func(a *plugintest.API) *plugintest.API {
				a.On("KVGet", avatarKey+testutils.GetID()).Return([]byte("mock data"), nil)
				return a
			},
			"",
		},
	}
	for _, tc := range ttcases {
		t.Run(tc.Name, func(t *testing.T) {
			assert := assert.New(t)
			s := SQLStore{}
			a := tc.SetupAPI(&plugintest.API{})
			s.api = a
			tc.SetupAPI(&plugintest.API{})
			resp, err := s.GetAvatarCache(testutils.GetID())

			if tc.ExpectedErrorMessage != "" {
				assert.Contains(err.Error(), tc.ExpectedErrorMessage)
				assert.Nil(resp)
			} else {
				assert.Nil(err)
				assert.NotNil(resp)
			}
		})
	}
}

func TestSetAvatarCache(t *testing.T) {
	ttcases := []struct {
		Name                 string
		SetupAPI             func(*plugintest.API) *plugintest.API
		ExpectedErrorMessage string
	}{
		{
			"SetAvatarCache: Error while setting the avatar cache",
			func(a *plugintest.API) *plugintest.API {
				a.On("KVSetWithExpiry", avatarKey+testutils.GetID(), []byte{10}, int64(avatarCacheTime)).Return(testutils.GetInternalServerAppError("unable to set the avatar cache"))
				return a
			},
			"unable to set the avatar cache",
		},
		{
			"SetAvatarCache: Valid",
			func(a *plugintest.API) *plugintest.API {
				a.On("KVSetWithExpiry", avatarKey+testutils.GetID(), []byte{10}, int64(avatarCacheTime)).Return(nil)
				return a
			},
			"",
		},
	}
	for _, tc := range ttcases {
		t.Run(tc.Name, func(t *testing.T) {
			assert := assert.New(t)
			s := SQLStore{}
			a := tc.SetupAPI(&plugintest.API{})
			s.api = a
			err := s.SetAvatarCache(testutils.GetID(), []byte{10})

			if tc.ExpectedErrorMessage != "" {
				assert.Contains(err.Error(), tc.ExpectedErrorMessage)
			} else {
				assert.Nil(err)
			}
		})
	}
}
