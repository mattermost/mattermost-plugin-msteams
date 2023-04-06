package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/oauth2"
)

func setupTestStore(api *plugintest.API) (*SQLStore, *plugintest.API, testcontainers.Container) {
	store := &SQLStore{}
	store.api = api
	store.driverName = "postgres"
	db, container := createTestDB()
	store.db = db
	_ = store.Init()
	return store, api, container
}

func createTestDB() (*sql.DB, testcontainers.Container) {
	postgresPort := nat.Port("5432/tcp")
	postgres, _ := testcontainers.GenericContainer(context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "postgres",
				ExposedPorts: []string{postgresPort.Port()},
				Env: map[string]string{
					"POSTGRES_PASSWORD": "pass",
					"POSTGRES_USER":     "user",
				},
				WaitingFor: wait.ForAll(
					wait.ForLog("database system is ready to accept connections"),
					wait.ForListeningPort(postgresPort),
				),
			},
			Started: true,
		})

	hostPort, _ := postgres.MappedPort(context.Background(), postgresPort)
	conn, _ := sqlx.Connect("postgres", fmt.Sprintf("postgres://user:pass@localhost:%s?sslmode=disable", hostPort.Port()))
	return conn.DB, postgres
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
			ExpectedErrorMessage: "",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			store, api, _ := setupTestStore(&plugintest.API{})
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
			ExpectedErrorMessage: "",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			store, api, _ := setupTestStore(&plugintest.API{})
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
			ExpectedResult: false,
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
			ExpectedResult: false,
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
			store, api, _ := setupTestStore(&plugintest.API{})
			test.SetupAPI(api)
			store.enabledTeams = test.EnabledTeams
			resp := store.CheckEnabledTeamByTeamID("mockTeamID")

			assert.Equal(test.ExpectedResult, resp)
		})
	}
}

func TestStoreChannelLinkAndGetLinkByChannelID(t *testing.T) {
	assert := assert.New(t)
	store, api, container := setupTestStore(&plugintest.API{})
	store.enabledTeams = func() []string { return []string{"mockMattermostTeamID"} }

	api.On("GetTeam", "mockMattermostTeamID").Return(&model.Team{
		Name: "mockMattermostTeamID",
	}, nil)

	mockChannelLink := &storemodels.ChannelLink{
		MattermostChannel: "mockMattermostChannelID",
		MattermostTeam:    "mockMattermostTeamID",
		MSTeamsTeam:       "mockMSTeamsTeamID",
		MSTeamsChannel:    "mockMSTeamsChannelID",
		Creator:           "mockCreator",
	}

	storeErr := store.StoreChannelLink(mockChannelLink)
	assert.Nil(storeErr)

	resp, getErr := store.GetLinkByChannelID("mockMattermostChannelID")
	assert.Equal(mockChannelLink, resp)
	assert.Nil(getErr)

	termErr := container.Terminate(context.Background())
	assert.Nil(termErr)
}

func TestStoreChannelLinkdAndGetLinkByMSTeamsChannelID(t *testing.T) {
	assert := assert.New(t)
	store, api, container := setupTestStore(&plugintest.API{})
	store.enabledTeams = func() []string { return []string{"mockMattermostTeamID"} }

	api.On("GetTeam", "mockMattermostTeamID").Return(&model.Team{
		Name: "mockMattermostTeamID",
	}, nil)

	mockChannelLink := &storemodels.ChannelLink{
		MattermostChannel: "mockMattermostChannelID",
		MattermostTeam:    "mockMattermostTeamID",
		MSTeamsTeam:       "mockMSTeamsTeamID",
		MSTeamsChannel:    "mockMSTeamsChannelID",
		Creator:           "mockCreator",
	}

	storeErr := store.StoreChannelLink(mockChannelLink)
	assert.Nil(storeErr)

	resp, getErr := store.GetLinkByMSTeamsChannelID("mockMattermostTeamID", "mockMSTeamsChannelID")
	assert.Equal(mockChannelLink, resp)
	assert.Nil(getErr)

	termErr := container.Terminate(context.Background())
	assert.Nil(termErr)
}

func TestStoreChannelLinkdAndDeleteLinkByChannelID(t *testing.T) {
	assert := assert.New(t)
	store, api, container := setupTestStore(&plugintest.API{})
	store.enabledTeams = func() []string { return []string{"mockMattermostTeamID"} }

	api.On("GetTeam", "mockMattermostTeamID").Return(&model.Team{
		Name: "mockMattermostTeamID",
	}, nil)

	mockChannelLink := &storemodels.ChannelLink{
		MattermostChannel: "mockMattermostChannelID",
		MattermostTeam:    "mockMattermostTeamID",
		MSTeamsTeam:       "mockMSTeamsTeamID",
		MSTeamsChannel:    "mockMSTeamsChannelID",
		Creator:           "mockCreator",
	}

	storeErr := store.StoreChannelLink(mockChannelLink)
	assert.Nil(storeErr)

	resp, getErr := store.GetLinkByChannelID("mockMattermostChannelID")
	assert.Equal(mockChannelLink, resp)
	assert.Nil(getErr)

	resp, getErr = store.GetLinkByMSTeamsChannelID("mockMattermostTeamID", "mockMSTeamsChannelID")
	assert.Equal(mockChannelLink, resp)
	assert.Nil(getErr)

	delErr := store.DeleteLinkByChannelID("mockMattermostChannelID")
	assert.Nil(delErr)

	resp, getErr = store.GetLinkByChannelID("mockMattermostChannelID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")

	resp, getErr = store.GetLinkByMSTeamsChannelID("mockMattermostTeamID", "mockMSTeamsChannelID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")

	termErr := container.Terminate(context.Background())
	assert.Nil(termErr)
}

func TestLinkPostsAndGetPostInfoByMSTeamsID(t *testing.T) {
	assert := assert.New(t)
	store, _, container := setupTestStore(&plugintest.API{})

	mockPostInfo := storemodels.PostInfo{
		MattermostID:        "mockMattermostID",
		MSTeamsID:           "mockMSTeamsID",
		MSTeamsChannel:      "mockMSTeamsChannel",
		MSTeamsLastUpdateAt: time.UnixMicro(int64(100)),
	}

	storeErr := store.LinkPosts(mockPostInfo)
	assert.Nil(storeErr)

	resp, getErr := store.GetPostInfoByMSTeamsID("mockMSTeamsChannel", "mockMSTeamsID")
	assert.Equal(&mockPostInfo, resp)
	assert.Nil(getErr)

	termErr := container.Terminate(context.Background())
	assert.Nil(termErr)
}

func TestLinkPostsAndGetPostInfoByMattermostID(t *testing.T) {
	assert := assert.New(t)
	store, _, container := setupTestStore(&plugintest.API{})

	mockPostInfo := storemodels.PostInfo{
		MattermostID:        "mockMattermostID",
		MSTeamsID:           "mockMSTeamsID",
		MSTeamsChannel:      "mockMSTeamsChannel",
		MSTeamsLastUpdateAt: time.UnixMicro(int64(100)),
	}

	storeErr := store.LinkPosts(mockPostInfo)
	assert.Nil(storeErr)

	resp, getErr := store.GetPostInfoByMattermostID("mockMattermostID")
	assert.Equal(&mockPostInfo, resp)
	assert.Nil(getErr)

	termErr := container.Terminate(context.Background())
	assert.Nil(termErr)
}

func TestSetUserInfoAndTeamsToMattermostUserID(t *testing.T) {
	assert := assert.New(t)
	store, _, container := setupTestStore(&plugintest.API{})
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	storeErr := store.SetUserInfo(testutils.GetID(), testutils.GetTeamUserID(), &oauth2.Token{})
	assert.Nil(storeErr)

	resp, getErr := store.TeamsToMattermostUserID(testutils.GetTeamUserID())
	assert.Equal(testutils.GetID(), resp)
	assert.Nil(getErr)

	termErr := container.Terminate(context.Background())
	assert.Nil(termErr)
}

func TestSetUserInfoAndMattermostToTeamsUserID(t *testing.T) {
	assert := assert.New(t)
	store, _, container := setupTestStore(&plugintest.API{})
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	storeErr := store.SetUserInfo(testutils.GetID(), testutils.GetTeamUserID(), &oauth2.Token{})
	assert.Nil(storeErr)

	resp, getErr := store.MattermostToTeamsUserID(testutils.GetID())
	assert.Equal(testutils.GetTeamUserID(), resp)
	assert.Nil(getErr)

	termErr := container.Terminate(context.Background())
	assert.Nil(termErr)
}

func TestSetUserInfoAndGetTokenForMattermostUser(t *testing.T) {
	assert := assert.New(t)
	store, _, container := setupTestStore(&plugintest.API{})
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	token := &oauth2.Token{
		AccessToken:  "mockAccessToken",
		RefreshToken: "mockRefreshToken",
	}

	storeErr := store.SetUserInfo(testutils.GetID(), testutils.GetTeamUserID(), token)
	assert.Nil(storeErr)

	resp, getErr := store.GetTokenForMattermostUser(testutils.GetID())
	assert.Equal(token, resp)
	assert.Nil(getErr)

	termErr := container.Terminate(context.Background())
	assert.Nil(termErr)
}

func TestSetUserInfoAndGetTokenForMSTeamsUser(t *testing.T) {
	assert := assert.New(t)
	store, _, container := setupTestStore(&plugintest.API{})
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	token := &oauth2.Token{
		AccessToken:  "mockAccessToken",
		RefreshToken: "mockRefreshToken",
	}

	storeErr := store.SetUserInfo(testutils.GetID(), testutils.GetTeamUserID(), token)
	assert.Nil(storeErr)

	resp, getErr := store.GetTokenForMSTeamsUser(testutils.GetTeamUserID())
	assert.Equal(token, resp)
	assert.Nil(getErr)

	termErr := container.Terminate(context.Background())
	assert.Nil(termErr)
}
