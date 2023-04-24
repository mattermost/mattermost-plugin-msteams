package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestStore(api *plugintest.API, driverName string) (*SQLStore, *plugintest.API, func()) {
	store := &SQLStore{}
	store.api = api
	store.driverName = driverName
	db, tearDownContainer := createTestDB(driverName)
	store.db = db
	_ = store.Init()
	return store, api, tearDownContainer
}

func createTestDB(driverName string) (*sql.DB, func()) {
	// Create postgres container
	if driverName == model.DatabaseDriverPostgres {
		postgresPort := nat.Port("5432/tcp")
		ctx := context.Background()
		postgres, _ := testcontainers.GenericContainer(ctx,
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

		ip, _ := postgres.Host(ctx)
		port, _ := postgres.MappedPort(ctx, "5432")
		conn, err := sqlx.Connect("postgres", fmt.Sprintf("postgres://user:pass@%s:%s?sslmode=disable", ip, port))
		if err != nil {
			log.Fatalf("failed to connect to the container: %s", err.Error())
		}
		tearDownContainer := func() {
			if err := postgres.Terminate(context.Background()); err != nil {
				log.Fatalf("failed to terminate container: %s", err.Error())
			}
		}

		return conn.DB, tearDownContainer
	}

	// Create MySQL container
	context := context.Background()
	mysql, _ := testcontainers.GenericContainer(context,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "mysql:latest",
				ExposedPorts: []string{"3306/tcp"},
				Env: map[string]string{
					"MYSQL_ROOT_PASSWORD": "root",
					"MYSQL_DATABASE":      "test",
				},
				WaitingFor: wait.ForAll(
					wait.ForLog("database system is ready to accept connections"),
				),
			},
			Started: true,
		})

	host, _ := mysql.Host(context)
	p, _ := mysql.MappedPort(context, "3306/tcp")
	port := p.Int()

	mysqlConn, _ := sqlx.Connect("mysql", fmt.Sprintf("root:root@tcp(%s:%d)/test", host, port))
	tearDownContainer := func() {
		if err := mysql.Terminate(context); err != nil {
			log.Fatalf("failed to terminate container: %s", err.Error())
		}
	}

	return mysqlConn.DB, tearDownContainer
}

func TestStore(t *testing.T) {
	testFunctions := map[string]func(*testing.T, *SQLStore, *plugintest.API){
		"testStoreChannelLinkAndGetLinkByChannelID":                  testStoreChannelLinkAndGetLinkByChannelID,
		"testGetLinkByChannelIDForInvalidID":                         testGetLinkByChannelIDForInvalidID,
		"testStoreChannelLinkdAndGetLinkByMSTeamsChannelID":          testStoreChannelLinkdAndGetLinkByMSTeamsChannelID,
		"testGetLinkByMSTeamsChannelIDForInvalidID":                  testGetLinkByMSTeamsChannelIDForInvalidID,
		"testStoreChannelLinkdAndDeleteLinkByChannelID":              testStoreChannelLinkdAndDeleteLinkByChannelID,
		"testDeleteLinkByChannelIDForInvalidID":                      testDeleteLinkByChannelIDForInvalidID,
		"testLinkPostsAndGetPostInfoByMSTeamsID":                     testLinkPostsAndGetPostInfoByMSTeamsID,
		"testGetPostInfoByMSTeamsIDForInvalidID":                     testGetPostInfoByMSTeamsIDForInvalidID,
		"testLinkPostsAndGetPostInfoByMattermostID":                  testLinkPostsAndGetPostInfoByMattermostID,
		"testGetPostInfoByMattermostIDForInvalidID":                  testGetPostInfoByMattermostIDForInvalidID,
		"testSetUserInfoAndTeamsToMattermostUserID":                  testSetUserInfoAndTeamsToMattermostUserID,
		"testTeamsToMattermostUserIDForInvalidID":                    testTeamsToMattermostUserIDForInvalidID,
		"testSetUserInfoAndMattermostToTeamsUserID":                  testSetUserInfoAndMattermostToTeamsUserID,
		"testMattermostToTeamsUserIDForInvalidID":                    testMattermostToTeamsUserIDForInvalidID,
		"testSetUserInfoAndGetTokenForMattermostUser":                testSetUserInfoAndGetTokenForMattermostUser,
		"testGetTokenForMattermostUserForInvalidUserID":              testGetTokenForMattermostUserForInvalidUserID,
		"testSetUserInfoAndGetTokenForMSTeamsUser":                   testSetUserInfoAndGetTokenForMSTeamsUser,
		"testGetTokenForMSTeamsUserForInvalidID":                     testGetTokenForMSTeamsUserForInvalidID,
		"testSetUserInfoAndGetTokenForMattermostUserWhereTokenIsNil": testSetUserInfoAndGetTokenForMattermostUserWhereTokenIsNil,
	}
	for _, driver := range []string{model.DatabaseDriverPostgres, model.DatabaseDriverMysql} {
		store, api, tearDownContainer := setupTestStore(&plugintest.API{}, driver)
		for test := range testFunctions {
			t.Run(driver+"/"+test, func(t *testing.T) {
				testFunctions[test](t, store, api)
			})
		}

		tearDownContainer()
	}
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
			store, api, tearDownContainer := setupTestStore(&plugintest.API{}, model.DatabaseDriverPostgres)
			defer tearDownContainer()
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
			store, api, tearDownContainer := setupTestStore(&plugintest.API{}, model.DatabaseDriverPostgres)
			defer tearDownContainer()
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
			store, api, tearDownContainer := setupTestStore(&plugintest.API{}, model.DatabaseDriverPostgres)
			defer tearDownContainer()
			test.SetupAPI(api)
			store.enabledTeams = test.EnabledTeams
			resp := store.CheckEnabledTeamByTeamID("mockTeamID")

			assert.Equal(test.ExpectedResult, resp)
		})
	}
}

func testStoreChannelLinkAndGetLinkByChannelID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)
	store.enabledTeams = func() []string { return []string{"mockMattermostTeamID-1"} }

	api.On("GetTeam", "mockMattermostTeamID-1").Return(&model.Team{
		Name: "mockMattermostTeamID-1",
	}, nil)

	mockChannelLink := &storemodels.ChannelLink{
		MattermostChannel: "mockMattermostChannelID-1",
		MattermostTeam:    "mockMattermostTeamID-1",
		MSTeamsTeam:       "mockMSTeamsTeamID-1",
		MSTeamsChannel:    "mockMSTeamsChannelID-1",
		Creator:           "mockCreator",
	}

	storeErr := store.StoreChannelLink(mockChannelLink)
	assert.Nil(storeErr)

	resp, getErr := store.GetLinkByChannelID("mockMattermostChannelID-1")
	assert.Equal(mockChannelLink, resp)
	assert.Nil(getErr)
}

func testGetLinkByChannelIDForInvalidID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.GetLinkByChannelID("invalidMattermostChannelID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testStoreChannelLinkdAndGetLinkByMSTeamsChannelID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)
	store.enabledTeams = func() []string { return []string{"mockMattermostTeamID-2"} }

	api.On("GetTeam", "mockMattermostTeamID-2").Return(&model.Team{
		Name: "mockMattermostTeamID-2",
	}, nil)

	mockChannelLink := &storemodels.ChannelLink{
		MattermostChannel: "mockMattermostChannelID-2",
		MattermostTeam:    "mockMattermostTeamID-2",
		MSTeamsTeam:       "mockMSTeamsTeamID-2",
		MSTeamsChannel:    "mockMSTeamsChannelID-2",
		Creator:           "mockCreator",
	}

	storeErr := store.StoreChannelLink(mockChannelLink)
	assert.Nil(storeErr)

	resp, getErr := store.GetLinkByMSTeamsChannelID("mockMattermostTeamID-2", "mockMSTeamsChannelID-2")
	assert.Equal(mockChannelLink, resp)
	assert.Nil(getErr)
}

func testGetLinkByMSTeamsChannelIDForInvalidID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.GetLinkByMSTeamsChannelID("invalidMattermostTeamID", "invalidMSTeamsChannelID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testStoreChannelLinkdAndDeleteLinkByChannelID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)
	store.enabledTeams = func() []string { return []string{"mockMattermostTeamID-3"} }

	api.On("GetTeam", "mockMattermostTeamID-3").Return(&model.Team{
		Name: "mockMattermostTeamID-3",
	}, nil)

	mockChannelLink := &storemodels.ChannelLink{
		MattermostChannel: "mockMattermostChannelID-3",
		MattermostTeam:    "mockMattermostTeamID-3",
		MSTeamsTeam:       "mockMSTeamsTeamID-3",
		MSTeamsChannel:    "mockMSTeamsChannelID-3",
		Creator:           "mockCreator",
	}

	storeErr := store.StoreChannelLink(mockChannelLink)
	assert.Nil(storeErr)

	resp, getErr := store.GetLinkByChannelID("mockMattermostChannelID-3")
	assert.Equal(mockChannelLink, resp)
	assert.Nil(getErr)

	resp, getErr = store.GetLinkByMSTeamsChannelID("mockMattermostTeamID-3", "mockMSTeamsChannelID-3")
	assert.Equal(mockChannelLink, resp)
	assert.Nil(getErr)

	delErr := store.DeleteLinkByChannelID("mockMattermostChannelID-3")
	assert.Nil(delErr)

	resp, getErr = store.GetLinkByChannelID("mockMattermostChannelID-3")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")

	resp, getErr = store.GetLinkByMSTeamsChannelID("mockMattermostTeamID-3", "mockMSTeamsChannelID-3")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testDeleteLinkByChannelIDForInvalidID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)

	delErr := store.DeleteLinkByChannelID("invalidIDMattermostChannelID")
	assert.Nil(delErr)
}

func testLinkPostsAndGetPostInfoByMSTeamsID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)

	mockPostInfo := storemodels.PostInfo{
		MattermostID:        "mockMattermostID-1",
		MSTeamsID:           "mockMSTeamsID-1",
		MSTeamsChannel:      "mockMSTeamsChannel-1",
		MSTeamsLastUpdateAt: time.UnixMicro(int64(100)),
	}

	storeErr := store.LinkPosts(mockPostInfo)
	assert.Nil(storeErr)

	resp, getErr := store.GetPostInfoByMSTeamsID("mockMSTeamsChannel-1", "mockMSTeamsID-1")
	assert.Equal(&mockPostInfo, resp)
	assert.Nil(getErr)
}

func testGetPostInfoByMSTeamsIDForInvalidID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.GetPostInfoByMSTeamsID("invalidMSTeamsChannel", "invalidMSTeamsID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testLinkPostsAndGetPostInfoByMattermostID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)

	mockPostInfo := storemodels.PostInfo{
		MattermostID:        "mockMattermostID-2",
		MSTeamsID:           "mockMSTeamsID-2",
		MSTeamsChannel:      "mockMSTeamsChannel-2",
		MSTeamsLastUpdateAt: time.UnixMicro(int64(100)),
	}

	storeErr := store.LinkPosts(mockPostInfo)
	assert.Nil(storeErr)

	resp, getErr := store.GetPostInfoByMattermostID("mockMattermostID-2")
	assert.Equal(&mockPostInfo, resp)
	assert.Nil(getErr)
}

func testGetPostInfoByMattermostIDForInvalidID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.GetPostInfoByMattermostID("invalidMattermostID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testSetUserInfoAndTeamsToMattermostUserID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"1", testutils.GetTeamUserID()+"1", &msteams.Token{})
	assert.Nil(storeErr)

	resp, getErr := store.TeamsToMattermostUserID(testutils.GetTeamUserID() + "1")
	assert.Equal(testutils.GetID()+"1", resp)
	assert.Nil(getErr)
}

func testTeamsToMattermostUserIDForInvalidID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.TeamsToMattermostUserID("invalidTeamsUserID")
	assert.Equal("", resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testSetUserInfoAndMattermostToTeamsUserID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"2", testutils.GetTeamUserID()+"2", &msteams.Token{})
	assert.Nil(storeErr)

	resp, getErr := store.MattermostToTeamsUserID(testutils.GetID() + "2")
	assert.Equal(testutils.GetTeamUserID()+"2", resp)
	assert.Nil(getErr)
}

func testMattermostToTeamsUserIDForInvalidID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.MattermostToTeamsUserID("invalidUserID")
	assert.Equal("", resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testSetUserInfoAndGetTokenForMattermostUser(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	token := &msteams.Token{
		Token:     "mockAccessToken-3",
		ExpiresOn: time.Now(),
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"3", testutils.GetTeamUserID()+"3", token)
	assert.Nil(storeErr)

	resp, getErr := store.GetTokenForMattermostUser(testutils.GetID() + "3")
	assert.Equal(token, resp)
	assert.Nil(getErr)
}

func testSetUserInfoAndGetTokenForMattermostUserWhereTokenIsNil(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"3", testutils.GetTeamUserID()+"3", nil)
	assert.Nil(storeErr)

	resp, getErr := store.GetTokenForMattermostUser(testutils.GetID() + "3")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "token not found")
}

func testGetTokenForMattermostUserForInvalidUserID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.GetTokenForMattermostUser("invalidUserID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testSetUserInfoAndGetTokenForMSTeamsUser(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	token := &msteams.Token{
		Token:     "mockAccessToken-4",
		ExpiresOn: time.Now(),
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"4", testutils.GetTeamUserID()+"4", token)
	assert.Nil(storeErr)

	resp, getErr := store.GetTokenForMSTeamsUser(testutils.GetTeamUserID() + "4")
	assert.Equal(token, resp)
	assert.Nil(getErr)
}

func testGetTokenForMSTeamsUserForInvalidID(t *testing.T, store *SQLStore, api *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.GetTokenForMSTeamsUser("invalidTeamsUserID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}
