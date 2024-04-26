package sqlstore

import (
	"fmt"
	"time"

	"testing"

	sq "github.com/Masterminds/squirrel"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

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
			store, api := setupTestStore(t)

			test.SetupAPI(api)
			store.enabledTeams = test.EnabledTeams
			resp := store.CheckEnabledTeamByTeamID("mockTeamID")

			assert.Equal(test.ExpectedResult, resp)
		})
	}
}

func TestStoreChannelLinkAndGetLinkByChannelID(t *testing.T) {
	store, api := setupTestStore(t)
	assert := assert.New(t)
	store.enabledTeams = func() []string { return []string{"mockMattermostTeamID-1"} }

	api.On("GetTeam", "mockMattermostTeamID-1").Return(&model.Team{
		Name: "mockMattermostTeamID-1",
	}, nil)

	mockChannelLink := &storemodels.ChannelLink{
		MattermostChannelID: "mockMattermostChannelID-1",
		MattermostTeamID:    "mockMattermostTeamID-1",
		MSTeamsTeam:         "mockMSTeamsTeamID-1",
		MSTeamsChannel:      "mockMSTeamsChannelID-1",
		Creator:             "mockCreator",
	}

	storeErr := store.StoreChannelLink(mockChannelLink)
	assert.Nil(storeErr)
	defer func() {
		_ = store.DeleteLinkByChannelID("mockMattermostChannelID-1")
	}()

	resp, getErr := store.GetLinkByChannelID("mockMattermostChannelID-1")
	assert.Equal(mockChannelLink, resp)
	assert.Nil(getErr)
}

func TestGetLinkByChannelIDForInvalidID(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)

	resp, getErr := store.GetLinkByChannelID("invalidMattermostChannelID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func TestStoreChannelLinkdAndGetLinkByMSTeamsChannelID(t *testing.T) {
	store, api := setupTestStore(t)
	assert := assert.New(t)
	store.enabledTeams = func() []string { return []string{"mockMattermostTeamID-2"} }

	api.On("GetTeam", "mockMattermostTeamID-2").Return(&model.Team{
		Name: "mockMattermostTeamID-2",
	}, nil)

	mockChannelLink := &storemodels.ChannelLink{
		MattermostChannelID: "mockMattermostChannelID-2",
		MattermostTeamID:    "mockMattermostTeamID-2",
		MSTeamsTeam:         "mockMSTeamsTeamID-2",
		MSTeamsChannel:      "mockMSTeamsChannelID-2",
		Creator:             "mockCreator",
	}

	storeErr := store.StoreChannelLink(mockChannelLink)
	assert.Nil(storeErr)
	defer func() {
		_ = store.DeleteLinkByChannelID("mockMattermostChannelID-2")
	}()

	resp, getErr := store.GetLinkByMSTeamsChannelID("mockMSTeamsTeamID-2", "mockMSTeamsChannelID-2")
	assert.Equal(mockChannelLink, resp)
	assert.Nil(getErr)
}

func TestGetLinkByMSTeamsChannelIDForInvalidID(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)

	resp, getErr := store.GetLinkByMSTeamsChannelID("invalidMattermostTeamID", "invalidMSTeamsChannelID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func TestStoreChannelLinkdAndDeleteLinkByChannelID(t *testing.T) {
	store, api := setupTestStore(t)
	assert := assert.New(t)
	store.enabledTeams = func() []string { return []string{"mockMattermostTeamID-3"} }

	api.On("GetTeam", "mockMattermostTeamID-3").Return(&model.Team{
		Name: "mockMattermostTeamID-3",
	}, nil)

	mockChannelLink := &storemodels.ChannelLink{
		MattermostChannelID: "mockMattermostChannelID-3",
		MattermostTeamID:    "mockMattermostTeamID-3",
		MSTeamsTeam:         "mockMSTeamsTeamID-3",
		MSTeamsChannel:      "mockMSTeamsChannelID-3",
		Creator:             "mockCreator",
	}

	storeErr := store.StoreChannelLink(mockChannelLink)
	assert.Nil(storeErr)
	defer func() {
		_ = store.DeleteLinkByChannelID("mockMattermostChannelID-3")
	}()

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

func TestListChannelLinksWithNames(t *testing.T) {
	store, api := setupTestStore(t)
	assert := assert.New(t)
	store.enabledTeams = func() []string { return []string{"mockMattermostTeamID-4"} }

	api.On("GetTeam", "mockMattermostTeamID-4").Return(&model.Team{
		Name: "mockMattermostTeamID-4",
	}, nil)

	mockChannelLink := &storemodels.ChannelLink{
		MattermostChannelID:   "mockMattermostChannelID-4",
		MattermostTeamID:      "mockMattermostTeamID-4",
		MattermostTeamName:    "Mock Mattermost Team",
		MattermostChannelName: "Mock Mattermost Channel",
		MSTeamsTeam:           "mockMSTeamsTeamID-4",
		MSTeamsChannel:        "mockMSTeamsChannelID-4",
		Creator:               "mockCreator",
	}

	_, err := store.getQueryBuilder().Insert("Teams").Columns("Id, DisplayName").Values(mockChannelLink.MattermostTeamID, mockChannelLink.MattermostTeamName).Exec()
	assert.Nil(err)
	_, err = store.getQueryBuilder().Insert("Channels").Columns("Id, DisplayName").Values(mockChannelLink.MattermostChannelID, mockChannelLink.MattermostChannelName).Exec()
	assert.Nil(err)

	links, err := store.ListChannelLinksWithNames()
	assert.Nil(err)
	assert.NotContains(links, mockChannelLink)

	storeErr := store.StoreChannelLink(mockChannelLink)
	assert.Nil(storeErr)
	defer func() {
		_ = store.DeleteLinkByChannelID("mockMattermostChannelID-4")
	}()

	links, err = store.ListChannelLinksWithNames()
	assert.Nil(err)
	assert.Contains(links, mockChannelLink)
}

func TestListChannelLinks(t *testing.T) {
	store, api := setupTestStore(t)
	store.enabledTeams = func() []string { return []string{"mockMattermostTeamID-1", "mockMattermostTeamID-2"} }

	api.On("GetTeam", "mockMattermostTeamID-1").Return(&model.Team{
		Name: "mockMattermostTeamID-1",
	}, nil)
	api.On("GetTeam", "mockMattermostTeamID-2").Return(&model.Team{
		Name: "mockMattermostTeamID-2",
	}, nil)

	links, err := store.ListChannelLinks()
	require.NoError(t, err)
	require.Len(t, links, 0)

	mockChannelLink := &storemodels.ChannelLink{
		MattermostChannelID: "mockMattermostChannelID-1",
		MattermostTeamID:    "mockMattermostTeamID-1",
		MSTeamsTeam:         "mockMSTeamsTeamID-1",
		MSTeamsChannel:      "mockMSTeamsChannelID-1",
		Creator:             "mockCreator",
	}

	err = store.StoreChannelLink(mockChannelLink)
	require.NoError(t, err)
	defer func() {
		_ = store.DeleteLinkByChannelID("mockMattermostChannelID-1")
	}()

	links, err = store.ListChannelLinks()
	require.NoError(t, err)
	require.Len(t, links, 1)

	mockChannelLink = &storemodels.ChannelLink{
		MattermostChannelID: "mockMattermostChannelID-2",
		MattermostTeamID:    "mockMattermostTeamID-2",
		MSTeamsTeam:         "mockMSTeamsTeamID-2",
		MSTeamsChannel:      "mockMSTeamsChannelID-2",
		Creator:             "mockCreator",
	}
	err = store.StoreChannelLink(mockChannelLink)
	require.NoError(t, err)
	defer func() {
		_ = store.DeleteLinkByChannelID("mockMattermostChannelID-2")
	}()

	links, err = store.ListChannelLinks()
	require.NoError(t, err)
	require.Len(t, links, 2)
}

func TestDeleteLinkByChannelIDForInvalidID(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)

	delErr := store.DeleteLinkByChannelID("invalidIDMattermostChannelID")
	assert.Nil(delErr)
}

func TestLinkPostsAndGetPostInfoByMSTeamsID(t *testing.T) {
	store, _ := setupTestStore(t)
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

func TestGetPostInfoByMSTeamsIDForInvalidID(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)

	resp, getErr := store.GetPostInfoByMSTeamsID("invalidMSTeamsChannel", "invalidMSTeamsID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func TestLinkPostsAndGetPostInfoByMattermostID(t *testing.T) {
	store, _ := setupTestStore(t)
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

func TestGetPostInfoByMattermostIDForInvalidID(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)

	resp, getErr := store.GetPostInfoByMattermostID("invalidMattermostID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func TestSetUserInfoAndTeamsToMattermostUserID(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"1", testutils.GetTeamsUserID()+"1", &oauth2.Token{})
	assert.Nil(storeErr)

	resp, getErr := store.TeamsToMattermostUserID(testutils.GetTeamsUserID() + "1")
	assert.Equal(testutils.GetID()+"1", resp)
	assert.Nil(getErr)

	deleteErr := store.DeleteUserInfo(testutils.GetID() + "1")
	assert.Nil(deleteErr)
}

func TestTeamsToMattermostUserIDForInvalidID(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)

	resp, getErr := store.TeamsToMattermostUserID("invalidTeamsUserID")
	assert.Equal("", resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func TestSetUserInfoAndMattermostToTeamsUserID(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"1", testutils.GetTeamsUserID()+"1", &oauth2.Token{})
	assert.Nil(storeErr)

	resp, getErr := store.MattermostToTeamsUserID(testutils.GetID() + "1")
	assert.Equal(testutils.GetTeamsUserID()+"1", resp)
	assert.Nil(getErr)

	delErr := store.DeleteUserInfo(testutils.GetID() + "1")
	assert.Nil(delErr)
}

func TestMattermostToTeamsUserIDForInvalidID(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)

	resp, getErr := store.MattermostToTeamsUserID("invalidUserID")
	assert.Equal("", resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func TestSetUserInfoAndGetTokenForMattermostUser(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	token := &oauth2.Token{
		AccessToken:  "mockAccessToken-1",
		RefreshToken: "mockRefreshToken-1",
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"1", testutils.GetTeamsUserID()+"1", token)
	assert.Nil(storeErr)

	resp, getErr := store.GetTokenForMattermostUser(testutils.GetID() + "1")
	assert.Equal(token, resp)
	assert.Nil(getErr)

	delErr := store.DeleteUserInfo(testutils.GetID() + "1")
	assert.Nil(delErr)
}

func TestSetUserInfoAndGetTokenForMattermostUserWhereTokenIsNil(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"1", testutils.GetTeamsUserID()+"1", nil)
	assert.Nil(storeErr)

	resp, getErr := store.GetTokenForMattermostUser(testutils.GetID() + "1")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")

	delErr := store.DeleteUserInfo(testutils.GetID() + "1")
	assert.Nil(delErr)
}

func TestGetTokenForMattermostUserForInvalidUserID(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)

	resp, getErr := store.GetTokenForMattermostUser("invalidUserID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func TestSetUserInfoAndGetTokenForMSTeamsUser(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	token := &oauth2.Token{
		AccessToken:  "mockAccessToken-4",
		RefreshToken: "mockRefreshToken-4",
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"1", testutils.GetTeamsUserID()+"1", token)
	assert.Nil(storeErr)

	resp, getErr := store.GetTokenForMSTeamsUser(testutils.GetTeamsUserID() + "1")
	assert.Equal(token, resp)
	assert.Nil(getErr)

	delErr := store.DeleteUserInfo(testutils.GetID() + "1")
	assert.Nil(delErr)
}

func TestGetTokenForMSTeamsUserForInvalidID(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)

	resp, getErr := store.GetTokenForMSTeamsUser("invalidTeamsUserID")
	assert.Nil(resp)
	assert.Contains(getErr.Error(), "no rows in result set")
}

func TestListGlobalSubscriptionsToCheck(t *testing.T) {
	store, _ := setupTestStore(t)
	t.Run("no-subscriptions", func(t *testing.T) {
		subscriptions, err := store.ListGlobalSubscriptionsToRefresh("")
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})

	t.Run("no-near-to-expire-subscriptions", func(t *testing.T) {
		err := store.SaveGlobalSubscription(testutils.GetGlobalSubscription("test", time.Now().Add(100*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test") }()

		subscriptions, err := store.ListGlobalSubscriptionsToRefresh("")
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})

	t.Run("almost-expired", func(t *testing.T) {
		err := store.SaveGlobalSubscription(testutils.GetGlobalSubscription("test1", time.Now().Add(2*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test1") }()

		subscriptions, err := store.ListGlobalSubscriptionsToRefresh("")
		require.NoError(t, err)
		require.Len(t, subscriptions, 1)
		assert.Equal(t, "test1", subscriptions[0].SubscriptionID)
	})

	t.Run("expired-subscription", func(t *testing.T) {
		err := store.SaveGlobalSubscription(testutils.GetGlobalSubscription("test1", time.Now().Add(-100*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test1") }()

		subscriptions, err := store.ListGlobalSubscriptionsToRefresh("")
		require.NoError(t, err)
		assert.Len(t, subscriptions, 1)
		assert.Equal(t, subscriptions[0].SubscriptionID, "test1")
	})
}

func TestListChatSubscriptionsToCheck(t *testing.T) {
	store, _ := setupTestStore(t)
	t.Run("no-subscriptions", func(t *testing.T) {
		subscriptions, err := store.ListChatSubscriptionsToCheck()
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})

	t.Run("no-near-to-expire-subscriptions", func(t *testing.T) {
		err := store.SaveChatSubscription(testutils.GetChatSubscription("test", "user-id", time.Now().Add(100*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test") }()

		subscriptions, err := store.ListChatSubscriptionsToCheck()
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})

	t.Run("multiple-subscriptions-with-different-expiry-dates", func(t *testing.T) {
		err := store.SaveChatSubscription(testutils.GetChatSubscription("test1", "user-id-1", time.Now().Add(100*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test1") }()
		err = store.SaveChatSubscription(testutils.GetChatSubscription("test2", "user-id-2", time.Now().Add(100*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test2") }()
		err = store.SaveChatSubscription(testutils.GetChatSubscription("test3", "user-id-3", time.Now().Add(100*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test3") }()
		err = store.SaveChatSubscription(testutils.GetChatSubscription("test4", "user-id-4", time.Now().Add(2*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test4") }()
		err = store.SaveChatSubscription(testutils.GetChatSubscription("test5", "user-id-5", time.Now().Add(2*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test5") }()
		err = store.SaveChatSubscription(testutils.GetChatSubscription("test6", "user-id-6", time.Now().Add(-100*time.Minute)))
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

func TestListChannelSubscriptionsToRefresh(t *testing.T) {
	store, _ := setupTestStore(t)
	t.Run("no-subscriptions", func(t *testing.T) {
		subscriptions, err := store.ListChannelSubscriptionsToRefresh("")
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})

	t.Run("no-near-to-expire-subscriptions", func(t *testing.T) {
		subscription := testutils.GetChannelSubscription("test", "team-id", "channel-id", time.Now().Add(100*time.Minute))
		go func() {
			err := store.SaveChannelSubscription(subscription)
			require.NoError(t, err)
		}()

		time.Sleep(1 * time.Second)
		_, err := store.GetChannelSubscription("test")
		require.NoError(t, err)

		defer func() { _ = store.DeleteSubscription("test") }()

		subscriptions, err := store.ListChannelSubscriptionsToRefresh("")
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})

	t.Run("multiple-subscriptions-with-different-expiry-dates", func(t *testing.T) {
		err := store.SaveChannelSubscription(testutils.GetChannelSubscription("test1", "team-id", "channel-id-1", time.Now().Add(100*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test1") }()
		err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test2", "team-id", "channel-id-2", time.Now().Add(100*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test2") }()
		err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test3", "team-id", "channel-id-3", time.Now().Add(100*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test3") }()
		err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test4", "team-id", "channel-id-4", time.Now().Add(2*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test4") }()
		err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test5", "team-id", "channel-id-5", time.Now().Add(2*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test5") }()
		err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test6", "team-id", "channel-id-6", time.Now().Add(-100*time.Minute)))
		require.NoError(t, err)
		defer func() { _ = store.DeleteSubscription("test6") }()

		subscriptions, err := store.ListChannelSubscriptionsToRefresh("")
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

func TestSaveGlobalSubscription(t *testing.T) {
	store, _ := setupTestStore(t)
	err := store.SaveGlobalSubscription(testutils.GetGlobalSubscription("test1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()
	err = store.SaveGlobalSubscription(testutils.GetGlobalSubscription("test2", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test2") }()

	subscriptions, err := store.ListGlobalSubscriptionsToRefresh("")
	require.NoError(t, err)
	require.Len(t, subscriptions, 1)
	assert.Equal(t, subscriptions[0].SubscriptionID, "test2")
}

func TestSaveChatSubscription(t *testing.T) {
	store, _ := setupTestStore(t)
	err := store.SaveChatSubscription(testutils.GetChatSubscription("test1", "user-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()
	err = store.SaveChatSubscription(testutils.GetChatSubscription("test2", "user-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test2") }()

	err = store.SaveChatSubscription(testutils.GetChatSubscription("test3", "user-2", time.Now().Add(100*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test3") }()
	err = store.SaveChatSubscription(testutils.GetChatSubscription("test4", "user-2", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test4") }()

	subscriptions, err := store.ListChatSubscriptionsToCheck()
	require.NoError(t, err)
	assert.Len(t, subscriptions, 2)
	assert.Contains(t, []string{subscriptions[0].SubscriptionID, subscriptions[1].SubscriptionID}, "test2")
	assert.Contains(t, []string{subscriptions[0].SubscriptionID, subscriptions[1].SubscriptionID}, "test4")
}

func TestSaveChannelSubscription(t *testing.T) {
	store, _ := setupTestStore(t)
	err := store.SaveChannelSubscription(testutils.GetChannelSubscription("test1", "team-id", "channel-id-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()
	err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test2", "team-id", "channel-id-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test2") }()

	err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test3", "team-id", "channel-id-2", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test3") }()
	err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test4", "team-id", "channel-id-2", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test4") }()

	subscriptions, err := store.ListChannelSubscriptionsToRefresh("")
	require.NoError(t, err)
	assert.Len(t, subscriptions, 2)
	assert.Contains(t, []string{subscriptions[0].SubscriptionID, subscriptions[1].SubscriptionID}, "test2")
	assert.Contains(t, []string{subscriptions[0].SubscriptionID, subscriptions[1].SubscriptionID}, "test4")
}

func TestUpdateSubscriptionExpiresOn(t *testing.T) {
	store, _ := setupTestStore(t)
	err := store.SaveChannelSubscription(testutils.GetChannelSubscription("test1", "team-id", "channel-id-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()

	subscriptions, err := store.ListChannelSubscriptionsToRefresh("")
	require.NoError(t, err)
	require.Len(t, subscriptions, 1)

	err = store.UpdateSubscriptionExpiresOn("test1", time.Now().Add(100*time.Minute))
	require.NoError(t, err)

	subscriptions, err = store.ListChannelSubscriptionsToRefresh("")
	require.NoError(t, err)
	require.Len(t, subscriptions, 0)

	err = store.UpdateSubscriptionExpiresOn("test1", time.Now().Add(2*time.Minute))
	require.NoError(t, err)

	subscriptions, err = store.ListChannelSubscriptionsToRefresh("")
	require.NoError(t, err)
	require.Len(t, subscriptions, 1)
}

func TestGetGlobalSubscription(t *testing.T) {
	store, _ := setupTestStore(t)
	err := store.SaveGlobalSubscription(testutils.GetGlobalSubscription("test1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()

	err = store.SaveChatSubscription(testutils.GetChatSubscription("test2", "user-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test2") }()
	err = store.SaveChatSubscription(testutils.GetChatSubscription("test3", "user-2", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test3") }()

	err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test4", "team-id", "channel-id-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test4") }()
	err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test5", "team-id", "channel-id-2", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test5") }()

	t.Run("not-existing-subscription", func(t *testing.T) {
		_, err := store.GetGlobalSubscription("not-valid")
		require.Error(t, err)
	})
	t.Run("not-global-subscription", func(t *testing.T) {
		_, err := store.GetGlobalSubscription("test3")
		require.Error(t, err)
		_, err = store.GetGlobalSubscription("test5")
		require.Error(t, err)
	})
	t.Run("global-subscription", func(t *testing.T) {
		subscription, err := store.GetGlobalSubscription("test1")
		require.NoError(t, err)
		assert.Equal(t, subscription.SubscriptionID, "test1")
	})
}

func TestGetChatSubscription(t *testing.T) {
	store, _ := setupTestStore(t)
	err := store.SaveGlobalSubscription(testutils.GetGlobalSubscription("test1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()

	err = store.SaveChatSubscription(testutils.GetChatSubscription("test2", "user-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test2") }()
	err = store.SaveChatSubscription(testutils.GetChatSubscription("test3", "user-2", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test3") }()

	err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test4", "team-id", "channel-id-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test4") }()
	err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test5", "team-id", "channel-id-2", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test5") }()

	t.Run("not-existing-subscription", func(t *testing.T) {
		_, err := store.GetChatSubscription("not-valid")
		require.Error(t, err)
	})
	t.Run("not-chat-subscription", func(t *testing.T) {
		_, err := store.GetChatSubscription("test1")
		require.Error(t, err)
		_, err = store.GetChatSubscription("test5")
		require.Error(t, err)
	})
	t.Run("chat-subscription", func(t *testing.T) {
		subscription, err := store.GetChatSubscription("test2")
		require.NoError(t, err)
		assert.Equal(t, subscription.SubscriptionID, "test2")
	})
}

func TestGetChannelSubscription(t *testing.T) {
	store, _ := setupTestStore(t)
	err := store.SaveGlobalSubscription(testutils.GetGlobalSubscription("test1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()

	err = store.SaveChatSubscription(testutils.GetChatSubscription("test2", "user-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test2") }()
	err = store.SaveChatSubscription(testutils.GetChatSubscription("test3", "user-2", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test3") }()

	err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test4", "team-id", "channel-id-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)

	defer func() { _ = store.DeleteSubscription("test4") }()

	err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test5", "team-id", "channel-id-2", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)

	defer func() { _ = store.DeleteSubscription("test5") }()

	t.Run("not-existing-subscription", func(t *testing.T) {
		_, err := store.GetChannelSubscription("not-valid")
		require.Error(t, err)
	})
	t.Run("not-channel-subscription", func(t *testing.T) {
		_, err := store.GetChannelSubscription("test1")
		require.Error(t, err)
		_, err = store.GetChannelSubscription("test3")
		require.Error(t, err)
	})
	t.Run("channel-subscription", func(t *testing.T) {
		subscription, err := store.GetChannelSubscription("test4")
		require.NoError(t, err)
		assert.Equal(t, subscription.SubscriptionID, "test4")
	})
}

func TestGetSubscriptionType(t *testing.T) {
	store, _ := setupTestStore(t)
	err := store.SaveGlobalSubscription(testutils.GetGlobalSubscription("test1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()

	err = store.SaveChatSubscription(testutils.GetChatSubscription("test2", "user-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test2") }()
	err = store.SaveChatSubscription(testutils.GetChatSubscription("test3", "user-2", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test3") }()

	err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test4", "team-id", "channel-id-1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)

	defer func() { _ = store.DeleteSubscription("test4") }()

	err = store.SaveChannelSubscription(testutils.GetChannelSubscription("test5", "team-id", "channel-id-2", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)

	defer func() { _ = store.DeleteSubscription("test5") }()

	t.Run("not-valid-subscription", func(t *testing.T) {
		_, err := store.GetChannelSubscription("not-valid")
		require.Error(t, err)
	})
	t.Run("global-subscription", func(t *testing.T) {
		subscriptionType, err := store.GetSubscriptionType("test1")
		require.NoError(t, err)
		assert.Equal(t, subscriptionType, subscriptionTypeAllChats)
	})
	t.Run("channel-subscription", func(t *testing.T) {
		subscriptionType, err := store.GetSubscriptionType("test4")
		require.NoError(t, err)
		assert.Equal(t, subscriptionType, subscriptionTypeChannel)
	})
	t.Run("chat-subscription", func(t *testing.T) {
		subscriptionType, err := store.GetSubscriptionType("test2")
		require.NoError(t, err)
		assert.Equal(t, subscriptionType, subscriptionTypeUser)
	})
}

func TestListChannelSubscriptions(t *testing.T) {
	store, _ := setupTestStore(t)
	err := store.SaveChannelSubscription(testutils.GetChannelSubscription("test1", "team-id", "channel-id", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)

	defer func() { _ = store.DeleteSubscription("test1") }()

	subscriptions, err := store.ListChannelSubscriptions()
	require.NoError(t, err)
	require.Len(t, subscriptions, 1)
}

func TestListGlobalSubscriptions(t *testing.T) {
	store, _ := setupTestStore(t)
	err := store.SaveGlobalSubscription(testutils.GetGlobalSubscription("test1", time.Now().Add(1*time.Minute)))
	require.NoError(t, err)
	defer func() { _ = store.DeleteSubscription("test1") }()

	subscriptions, err := store.ListGlobalSubscriptions()
	require.NoError(t, err)
	require.Len(t, subscriptions, 1)
}

func TestStoreAndVerifyOAuthState(t *testing.T) {
	store, api := setupTestStore(t)
	assert := assert.New(t)
	store.enabledTeams = func() []string { return []string{"mockMattermostTeamID-1"} }

	api.On("GetTeam", "mockMattermostTeamID-1").Return(&model.Team{
		Name: "mockMattermostTeamID-1",
	}, nil)

	state := fmt.Sprintf("%s_%s", model.NewId(), "mockMattermostUserID")
	key := hashKey(oAuth2KeyPrefix, state)
	api.On("KVSetWithExpiry", key, []byte(state), int64(oAuth2StateTimeToLive)).Return(nil)
	err := store.StoreOAuth2State(state)
	assert.Nil(err)

	api.On("KVGet", key).Return([]byte(state), nil)
	err = store.VerifyOAuth2State(state)
	assert.Nil(err)
}

func TestListConnectedUsers(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)
	store.encryptionKey = func() []byte {
		return make([]byte, 16)
	}

	token := &oauth2.Token{
		AccessToken:  "mockAccessToken-1",
		RefreshToken: "mockRefreshToken-1",
	}

	storeErr := store.SetUserInfo(testutils.GetID()+"1", testutils.GetTeamsUserID()+"1", token)
	assert.Nil(storeErr)

	storeErr = store.SetUserInfo(testutils.GetID()+"2", testutils.GetTeamsUserID()+"2", nil)
	assert.Nil(storeErr)

	_, err := store.getQueryBuilder().Insert("Users").Columns("Id, Email, FirstName, LastName").Values(testutils.GetID()+"1", testutils.GetTestEmail(), "mockFirstName", "mockLastName").Exec()
	assert.Nil(err)

	resp, getErr := store.GetConnectedUsers(0, 100)
	expectedResp := []*storemodels.ConnectedUser{
		{
			MattermostUserID: testutils.GetID() + "1",
			TeamsUserID:      testutils.GetTeamsUserID() + "1",
			FirstName:        "mockFirstName",
			LastName:         "mockLastName",
			Email:            testutils.GetTestEmail(),
		},
	}

	assert.Equal(expectedResp, resp)
	assert.Nil(getErr)

	delErr := store.DeleteUserInfo(testutils.GetID() + "1")
	assert.Nil(delErr)

	delErr = store.DeleteUserInfo(testutils.GetID() + "2")
	assert.Nil(delErr)
}

func TestWhitelistIO(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := assert.New(t)

	count, getErr := store.GetWhitelistCount()
	assert.Equal(0, count)
	assert.Nil(getErr)

	storeErr := store.StoreUserInWhitelist(testutils.GetUserID() + "1")
	assert.Nil(storeErr)

	count, getErr = store.GetWhitelistCount()
	assert.Equal(1, count)
	assert.Nil(getErr)

	present, presentErr := store.IsUserWhitelisted(testutils.GetUserID() + "1")
	assert.Equal(true, present)
	assert.Nil(presentErr)

	present, presentErr = store.IsUserWhitelisted(testutils.GetTeamsUserID() + "1")
	assert.Equal(false, present)
	assert.Nil(presentErr)

	storeErr = store.StoreUserInWhitelist(testutils.GetUserID() + "2")
	assert.Nil(storeErr)

	count, getErr = store.GetWhitelistCount()
	assert.Equal(2, count)
	assert.Nil(getErr)

	present, presentErr = store.IsUserWhitelisted(testutils.GetUserID() + "2")
	assert.Equal(true, present)
	assert.Nil(presentErr)

	tx, txErr := store.db.Begin()
	assert.Nil(txErr)
	delErr := store.deleteWhitelist(tx)
	assert.Nil(delErr)
	txCommitErr := tx.Commit()
	assert.Nil(txCommitErr)

	count, getErr = store.GetWhitelistCount()
	assert.Equal(0, count)
	assert.Nil(getErr)
}

func TestSetUserLastChatSentAt(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := require.New(t)

	mmUserID := model.NewId()
	err := store.SetUserInfo(mmUserID, "ms-"+mmUserID, nil)
	assert.Nil(err)

	getLastChatSentAtForUser := func(mmUserID string) int64 {
		t.Helper()
		var lastChatSentAt int64
		err = store.getQueryBuilder().
			Select("LastChatSentAt").
			From(usersTableName).
			Where(sq.Eq{"mmuseriD": mmUserID}).
			QueryRow().
			Scan(&lastChatSentAt)
		assert.Nil(err)
		return lastChatSentAt
	}

	{
		// Initial SetUserLastChatSentAt
		err = store.SetUserLastChatSentAt(mmUserID, 10)
		assert.Nil(err)
		assert.EqualValues(10, getLastChatSentAtForUser(mmUserID))
	}
	{
		// Don't update if sentAt is less than current
		err = store.SetUserLastChatSentAt(mmUserID, 5)
		assert.Nil(err)
		assert.EqualValues(10, getLastChatSentAtForUser(mmUserID))
	}
	{
		// Update if sentAt is greater than current
		err = store.SetUserLastChatSentAt(mmUserID, 15)
		assert.Nil(err)
		assert.EqualValues(15, getLastChatSentAtForUser(mmUserID))
	}
}

func TestSetUsersLastChatReceivedAt(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := require.New(t)

	mmUserID1 := model.NewId()
	err := store.SetUserInfo(mmUserID1, "ms-"+mmUserID1, nil)
	assert.Nil(err)
	mmUserID2 := model.NewId()
	err = store.SetUserInfo(mmUserID2, "ms-"+mmUserID2, nil)
	assert.Nil(err)
	mmUserID3 := model.NewId()
	err = store.SetUserInfo(mmUserID3, "ms-"+mmUserID3, nil)
	assert.Nil(err)
	allMMUsers := []string{mmUserID1, mmUserID2, mmUserID3}

	getLastChatReceivedAtForUser := func(mmUserID string) int64 {
		t.Helper()
		var lastChatReceivedAt int64
		err = store.getQueryBuilder().
			Select("LastChatReceivedAt").
			From(usersTableName).
			Where(sq.Eq{"mmuseriD": mmUserID}).
			QueryRow().
			Scan(&lastChatReceivedAt)
		assert.Nil(err)
		return lastChatReceivedAt
	}

	{
		// Initial SetUsersLastChatReceivedAt
		err = store.SetUsersLastChatReceivedAt(allMMUsers, 10)
		assert.Nil(err)
		for _, mmUserID := range allMMUsers {
			assert.EqualValues(10, getLastChatReceivedAtForUser(mmUserID))
		}
	}
	{
		// Don't update if receivedAt is less than current
		err = store.SetUsersLastChatReceivedAt(allMMUsers, 5)
		assert.Nil(err)
		for _, mmUserID := range allMMUsers {
			assert.EqualValues(10, getLastChatReceivedAtForUser(mmUserID))
		}
	}
	{
		// Update if sentAt is greater than current
		err = store.SetUsersLastChatReceivedAt(allMMUsers, 15)
		assert.Nil(err)
		for _, mmUserID := range allMMUsers {
			assert.EqualValues(15, getLastChatReceivedAtForUser(mmUserID))
		}
	}
	{
		// Update if sentAt is greater than current for some users
		// u2 will have 25, u1 and u3 will have 20.
		// Updating them all to 22 will result in u1 and u3 having 22
		// but 2 should keep its 25
		err = store.SetUserLastChatReceivedAt(mmUserID2, 25)
		assert.Nil(err)
		assert.EqualValues(25, getLastChatReceivedAtForUser(mmUserID2))

		err = store.SetUsersLastChatReceivedAt([]string{mmUserID1, mmUserID3}, 20)
		assert.Nil(err)
		assert.EqualValues(20, getLastChatReceivedAtForUser(mmUserID1))
		assert.EqualValues(20, getLastChatReceivedAtForUser(mmUserID3))

		err = store.SetUsersLastChatReceivedAt(allMMUsers, 22)
		assert.Nil(err)
		assert.EqualValues(22, getLastChatReceivedAtForUser(mmUserID1))
		assert.EqualValues(22, getLastChatReceivedAtForUser(mmUserID3))
		assert.EqualValues(25, getLastChatReceivedAtForUser(mmUserID2))
	}
}

func TestGetExtraStats(t *testing.T) {
	store, _ := setupTestStore(t)
	assert := require.New(t)

	// reset all the stats
	_, err := store.getQueryBuilder().Update(usersTableName).
		Set("LastChatReceivedAt", 0).
		Set("LastChatSentAt", 0).
		Exec()
	assert.Nil(err)

	users := []string{model.NewId(), model.NewId(), model.NewId()}
	for _, mmUserID := range users {
		err = store.SetUserInfo(mmUserID, "teams-"+mmUserID, nil)
		assert.Nil(err)
	}
	// Give them all a last chat received at in the test range
	err = store.SetUsersLastChatReceivedAt(users, 25)
	assert.Nil(err)

	// Have user 2 and 3 sent at be in the range
	err = store.SetUserLastChatSentAt(users[0], 10)
	assert.Nil(err)
	err = store.SetUserLastChatSentAt(users[1], 20)
	assert.Nil(err)
	err = store.SetUserLastChatSentAt(users[2], 30)
	assert.Nil(err)

	stats := &storemodels.Stats{}
	from := time.UnixMicro(19)
	to := time.UnixMicro(35)
	err = store.GetExtraStats(stats, from, to)
	assert.Nil(err)

	assert.EqualValues(2, stats.ActiveUsersSending)
	assert.EqualValues(3, stats.ActiveUsersReceiving)
}
