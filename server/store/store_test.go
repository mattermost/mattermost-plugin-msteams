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
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/oauth2"
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
		"testListGlobalSubscriptionsToCheck":                         testListGlobalSubscriptionsToCheck,
		"testListChatSubscriptionsToCheck":                           testListChatSubscriptionsToCheck,
		"testListChannelSubscriptionsToCheck":                        testListChannelSubscriptionsToCheck,
		"testSaveGlobalSubscription":                                 testSaveGlobalSubscription,
		"testSaveChatSubscription":                                   testSaveChatSubscription,
		"testSaveChannelSubscription":                                testSaveChannelSubscription,
		"testUpdateSubscriptionExpiresOn":                            testUpdateSubscriptionExpiresOn,
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

func testGetLinkByChannelIDForInvalidID(t *testing.T, store *SQLStore, _ *plugintest.API) {
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

	resp, getErr := store.GetLinkByMSTeamsChannelID("mockMSTeamsTeamID-2", "mockMSTeamsChannelID-2")
	assert.Equal(mockChannelLink, resp)
	assert.Nil(getErr)
}

func testGetLinkByMSTeamsChannelIDForInvalidID(t *testing.T, store *SQLStore, _ *plugintest.API) {
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

	resp, getErr = store.GetLinkByMSTeamsChannelID("mockMSTeamsTeamID-3", "mockMSTeamsChannelID-3")
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

func testDeleteLinkByChannelIDForInvalidID(t *testing.T, store *SQLStore, _ *plugintest.API) {
	assert := assert.New(t)

	delErr := store.DeleteLinkByChannelID("invalidIDMattermostChannelID")
	assert.Nil(delErr)
}

func testLinkPostsAndGetPostInfoByMSTeamsID(t *testing.T, store *SQLStore, _ *plugintest.API) {
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

func testGetPostInfoByMSTeamsIDForInvalidID(t *testing.T, store *SQLStore, _ *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.GetPostInfoByMSTeamsID("invalidMSTeamsChannel", "invalidMSTeamsID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testLinkPostsAndGetPostInfoByMattermostID(t *testing.T, store *SQLStore, _ *plugintest.API) {
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

func testGetPostInfoByMattermostIDForInvalidID(t *testing.T, store *SQLStore, _ *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.GetPostInfoByMattermostID("invalidMattermostID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testSetUserInfoAndTeamsToMattermostUserID(t *testing.T, store *SQLStore, _ *plugintest.API) {
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"1", testutils.GetTeamUserID()+"1", &oauth2.Token{})
	assert.Nil(storeErr)

	resp, getErr := store.TeamsToMattermostUserID(testutils.GetTeamUserID() + "1")
	assert.Equal(testutils.GetID()+"1", resp)
	assert.Nil(getErr)
}

func testTeamsToMattermostUserIDForInvalidID(t *testing.T, store *SQLStore, _ *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.TeamsToMattermostUserID("invalidTeamsUserID")
	assert.Equal("", resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testSetUserInfoAndMattermostToTeamsUserID(t *testing.T, store *SQLStore, _ *plugintest.API) {
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"2", testutils.GetTeamUserID()+"2", &oauth2.Token{})
	assert.Nil(storeErr)

	resp, getErr := store.MattermostToTeamsUserID(testutils.GetID() + "2")
	assert.Equal(testutils.GetTeamUserID()+"2", resp)
	assert.Nil(getErr)
}

func testMattermostToTeamsUserIDForInvalidID(t *testing.T, store *SQLStore, _ *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.MattermostToTeamsUserID("invalidUserID")
	assert.Equal("", resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testSetUserInfoAndGetTokenForMattermostUser(t *testing.T, store *SQLStore, _ *plugintest.API) {
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	token := &oauth2.Token{
		AccessToken:  "mockAccessToken-3",
		RefreshToken: "mockRefreshToken-3",
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"3", testutils.GetTeamUserID()+"3", token)
	assert.Nil(storeErr)

	resp, getErr := store.GetTokenForMattermostUser(testutils.GetID() + "3")
	assert.Equal(token, resp)
	assert.Nil(getErr)
}

func testSetUserInfoAndGetTokenForMattermostUserWhereTokenIsNil(t *testing.T, store *SQLStore, _ *plugintest.API) {
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

func testGetTokenForMattermostUserForInvalidUserID(t *testing.T, store *SQLStore, _ *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.GetTokenForMattermostUser("invalidUserID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testSetUserInfoAndGetTokenForMSTeamsUser(t *testing.T, store *SQLStore, _ *plugintest.API) {
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	token := &oauth2.Token{
		AccessToken:  "mockAccessToken-4",
		RefreshToken: "mockRefreshToken-4",
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"4", testutils.GetTeamUserID()+"4", token)
	assert.Nil(storeErr)

	resp, getErr := store.GetTokenForMSTeamsUser(testutils.GetTeamUserID() + "4")
	assert.Equal(token, resp)
	assert.Nil(getErr)
}

func testGetTokenForMSTeamsUserForInvalidID(t *testing.T, store *SQLStore, _ *plugintest.API) {
	assert := assert.New(t)

	resp, getErr := store.GetTokenForMSTeamsUser("invalidTeamsUserID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func testListGlobalSubscriptionsToCheck(t *testing.T, store *SQLStore, _ *plugintest.API) {
	t.Run("no-subscriptions", func(t *testing.T) {
		subscriptions, err := store.ListGlobalSubscriptionsToCheck()
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})

	t.Run("no-near-to-expire-subscriptions", func(t *testing.T) {
		err := store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: "test", Type: "allChannels", Secret: "secret", ExpiresOn: time.Now().Add(100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test") }()

		subscriptions, err := store.ListGlobalSubscriptionsToCheck()
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})

	t.Run("one-subscription-almost-expired-and-other-dont", func(t *testing.T) {
		err := store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: "test1", Type: "allChannels", Secret: "secret", ExpiresOn: time.Now().Add(100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test1") }()
		err = store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: "test2", Type: "allChats", Secret: "secret", ExpiresOn: time.Now().Add(2 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test2") }()

		subscriptions, err := store.ListGlobalSubscriptionsToCheck()
		require.NoError(t, err)
		require.Len(t, subscriptions, 1)
		assert.Equal(t, "test2", subscriptions[0].SubscriptionID)
	})

	t.Run("one-subscription-almost-expired-and-other-expired", func(t *testing.T) {
		err := store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: "test1", Type: "allChannels", Secret: "secret", ExpiresOn: time.Now().Add(-100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test1") }()
		err = store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: "test2", Type: "allChats", Secret: "secret", ExpiresOn: time.Now().Add(2 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test2") }()

		subscriptions, err := store.ListGlobalSubscriptionsToCheck()
		require.NoError(t, err)
		assert.Len(t, subscriptions, 2)
		assert.Contains(t, []string{subscriptions[0].SubscriptionID, subscriptions[1].SubscriptionID}, "test1")
		assert.Contains(t, []string{subscriptions[0].SubscriptionID, subscriptions[1].SubscriptionID}, "test2")
	})
}

func testListChatSubscriptionsToCheck(t *testing.T, store *SQLStore, _ *plugintest.API) {
	t.Run("no-subscriptions", func(t *testing.T) {
		subscriptions, err := store.ListChatSubscriptionsToCheck()
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})

	t.Run("no-near-to-expire-subscriptions", func(t *testing.T) {
		err := store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: "test", UserID: "user-id", Secret: "secret", ExpiresOn: time.Now().Add(100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test") }()

		subscriptions, err := store.ListChatSubscriptionsToCheck()
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})

	t.Run("multiple-subscriptions-with-different-expiry-dates", func(t *testing.T) {
		err := store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: "test1", UserID: "user-id-1", Secret: "secret", ExpiresOn: time.Now().Add(100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test1") }()
		err = store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: "test2", UserID: "user-id-2", Secret: "secret", ExpiresOn: time.Now().Add(100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test2") }()
		err = store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: "test3", UserID: "user-id-3", Secret: "secret", ExpiresOn: time.Now().Add(100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test3") }()
		err = store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: "test4", UserID: "user-id-4", Secret: "secret", ExpiresOn: time.Now().Add(2 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test4") }()
		err = store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: "test5", UserID: "user-id-5", Secret: "secret", ExpiresOn: time.Now().Add(2 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test5") }()
		err = store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: "test6", UserID: "user-id-6", Secret: "secret", ExpiresOn: time.Now().Add(-100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test6") }()

		subscriptions, err := store.ListChatSubscriptionsToCheck()
		require.NoError(t, err)
		assert.Len(t, subscriptions, 3)
		ids := []string{}
		for _, s := range subscriptions {
			ids = append(ids, s.SubscriptionID)
		}
		assert.NotContains(t, ids, "test1")
		assert.NotContains(t, ids, "test2")
		assert.NotContains(t, ids, "test3")
		assert.Contains(t, ids, "test4")
		assert.Contains(t, ids, "test5")
		assert.Contains(t, ids, "test6")
	})
}

func testListChannelSubscriptionsToCheck(t *testing.T, store *SQLStore, _ *plugintest.API) {
	t.Run("no-subscriptions", func(t *testing.T) {
		subscriptions, err := store.ListChannelSubscriptionsToCheck()
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})

	t.Run("no-near-to-expire-subscriptions", func(t *testing.T) {
		err := store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: "test", TeamID: "team-id", ChannelID: "channel-id", Secret: "secret", ExpiresOn: time.Now().Add(100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test") }()

		subscriptions, err := store.ListChannelSubscriptionsToCheck()
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})

	t.Run("multiple-subscriptions-with-different-expiry-dates", func(t *testing.T) {
		err := store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: "test1", TeamID: "team-id", ChannelID: "channel-id-1", Secret: "secret", ExpiresOn: time.Now().Add(100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test1") }()
		err = store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: "test2", TeamID: "team-id", ChannelID: "channel-id-2", Secret: "secret", ExpiresOn: time.Now().Add(100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test2") }()
		err = store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: "test3", TeamID: "team-id", ChannelID: "channel-id-3", Secret: "secret", ExpiresOn: time.Now().Add(100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test3") }()
		err = store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: "test4", TeamID: "team-id", ChannelID: "channel-id-4", Secret: "secret", ExpiresOn: time.Now().Add(2 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test4") }()
		err = store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: "test5", TeamID: "team-id", ChannelID: "channel-id-5", Secret: "secret", ExpiresOn: time.Now().Add(2 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test5") }()
		err = store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: "test6", TeamID: "team-id", ChannelID: "channel-id-6", Secret: "secret", ExpiresOn: time.Now().Add(-100 * time.Minute)})
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test6") }()

		subscriptions, err := store.ListChannelSubscriptionsToCheck()
		require.NoError(t, err)
		assert.Len(t, subscriptions, 3)
		ids := []string{}
		for _, s := range subscriptions {
			ids = append(ids, s.SubscriptionID)
		}
		assert.NotContains(t, ids, "test1")
		assert.NotContains(t, ids, "test2")
		assert.NotContains(t, ids, "test3")
		assert.Contains(t, ids, "test4")
		assert.Contains(t, ids, "test5")
		assert.Contains(t, ids, "test6")
	})
}
func testSaveGlobalSubscription(t *testing.T, store *SQLStore, _ *plugintest.API) {
	err := store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: "test1", Type: "allChannels", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()
	err = store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: "test2", Type: "allChannels", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test2") }()

	err = store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: "test3", Type: "allChats", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test3") }()
	err = store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: "test4", Type: "allChats", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test4") }()

	subscriptions, err := store.ListGlobalSubscriptionsToCheck()
	require.NoError(t, err)
	require.Len(t, subscriptions, 2)
	assert.Contains(t, []string{subscriptions[0].SubscriptionID, subscriptions[1].SubscriptionID}, "test2")
	assert.Contains(t, []string{subscriptions[0].SubscriptionID, subscriptions[1].SubscriptionID}, "test4")
}

func testSaveChatSubscription(t *testing.T, store *SQLStore, _ *plugintest.API) {
	err := store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: "test1", UserID: "user-1", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()
	err = store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: "test2", UserID: "user-1", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test2") }()

	err = store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: "test3", UserID: "user-2", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test3") }()
	err = store.SaveChatSubscription(storemodels.ChatSubscription{SubscriptionID: "test4", UserID: "user-2", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test4") }()

	subscriptions, err := store.ListChatSubscriptionsToCheck()
	require.NoError(t, err)
	assert.Len(t, subscriptions, 2)
	assert.Contains(t, []string{subscriptions[0].SubscriptionID, subscriptions[1].SubscriptionID}, "test2")
	assert.Contains(t, []string{subscriptions[0].SubscriptionID, subscriptions[1].SubscriptionID}, "test4")
}

func testSaveChannelSubscription(t *testing.T, store *SQLStore, _ *plugintest.API) {
	err := store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: "test1", TeamID: "team-id", ChannelID: "channel-id-1", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()
	err = store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: "test2", TeamID: "team-id", ChannelID: "channel-id-1", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test2") }()

	err = store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: "test3", TeamID: "team-id", ChannelID: "channel-id-2", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test3") }()
	err = store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: "test4", TeamID: "team-id", ChannelID: "channel-id-2", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test4") }()

	subscriptions, err := store.ListChannelSubscriptionsToCheck()
	require.NoError(t, err)
	assert.Len(t, subscriptions, 2)
	assert.Contains(t, []string{subscriptions[0].SubscriptionID, subscriptions[1].SubscriptionID}, "test2")
	assert.Contains(t, []string{subscriptions[0].SubscriptionID, subscriptions[1].SubscriptionID}, "test4")
}

func testUpdateSubscriptionExpiresOn(t *testing.T, store *SQLStore, _ *plugintest.API) {
	err := store.SaveChannelSubscription(storemodels.ChannelSubscription{SubscriptionID: "test1", TeamID: "team-id", ChannelID: "channel-id-1", Secret: "secret", ExpiresOn: time.Now().Add(1 * time.Minute)})
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()

	subscriptions, err := store.ListChannelSubscriptionsToCheck()
	require.NoError(t, err)
	require.Len(t, subscriptions, 1)

	err = store.UpdateSubscriptionExpiresOn("test1", time.Now().Add(100*time.Minute))
	require.NoError(t, err)

	subscriptions, err = store.ListChannelSubscriptionsToCheck()
	require.NoError(t, err)
	require.Len(t, subscriptions, 0)

	err = store.UpdateSubscriptionExpiresOn("test1", time.Now().Add(2*time.Minute))
	require.NoError(t, err)

	subscriptions, err = store.ListChannelSubscriptionsToCheck()
	require.NoError(t, err)
	require.Len(t, subscriptions, 1)
}
