package handlers

import (
	"net/http"
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	storemocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type pluginMock struct {
	api                plugin.API
	store              store.Store
	syncDirectMessages bool
	botUserID          string
	url                string
	appClient          msteams.Client
	userClient         msteams.Client
	teamsUserClient    msteams.Client
}

func (pm *pluginMock) GetAPI() plugin.API                              { return pm.api }
func (pm *pluginMock) GetStore() store.Store                           { return pm.store }
func (pm *pluginMock) GetSyncDirectMessages() bool                     { return pm.syncDirectMessages }
func (pm *pluginMock) GetBotUserID() string                            { return pm.botUserID }
func (pm *pluginMock) GetURL() string                                  { return pm.url }
func (pm *pluginMock) GetClientForApp() msteams.Client                 { return pm.appClient }
func (pm *pluginMock) GetClientForUser(string) (msteams.Client, error) { return pm.userClient, nil }
func (pm *pluginMock) GetClientForTeamsUser(string) (msteams.Client, error) {
	return pm.teamsUserClient, nil
}

func newTestHandler() *ActivityHandler {
	return New(&pluginMock{
		appClient:          &mocks.Client{},
		userClient:         &mocks.Client{},
		teamsUserClient:    &mocks.Client{},
		store:              &storemocks.Store{},
		api:                &plugintest.API{},
		botUserID:          "bot-user-id",
		url:                "fake-url",
		syncDirectMessages: false,
	})
}

func TestGetOrCreateSyntheticUser(t *testing.T) {
	for _, test := range []struct {
		Name           string
		UserID         string
		DisplayName    string
		SetupStore     func(*storemocks.Store)
		SetupAPI       func(*plugintest.API)
		SetupAppClient func(*mocks.Client)
		ExpectedResult string
		ExpectedError  bool
	}{
		{
			Name:        "Uknown user but maching an already existing synthetic user",
			UserID:      "unknown-user",
			DisplayName: "Unkown User",
			SetupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", "unknown-user").Return("", nil).Times(1)
				store.On("SetUserInfo", "new-user-id", "unknown-user", mock.Anything).Return(nil).Times(1)
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByEmail", "unknown-user@msteamssync").Return(&model.User{Id: "new-user-id"}, nil).Times(1)
			},
			SetupAppClient: func(*mocks.Client) {},
			ExpectedResult: "new-user-id",
		},
		{
			Name:        "Uknown user without display name not maching an already existing synthetic user",
			UserID:      "unknown-user",
			DisplayName: "",
			SetupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", "unknown-user").Return("", nil).Times(1)
				store.On("SetUserInfo", "new-user-id", "unknown-user", mock.Anything).Return(nil).Times(1)
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByEmail", "unknown-user@msteamssync").Return(nil, model.NewAppError("test", "not-found", nil, "", http.StatusNotFound)).Times(1)
				api.On("CreateUser", mock.MatchedBy(func(user *model.User) bool {
					if user.Username != "new-display-name-unknown-user" {
						return false
					}
					if user.FirstName != "New display name" {
						return false
					}
					if user.Email != "unknown-user@msteamssync" {
						return false
					}
					return true
				})).Return(&model.User{Id: "new-user-id"}, nil).Times(1)
			},
			SetupAppClient: func(client *mocks.Client) {
				client.On("GetUser", "unknown-user").Return(&msteams.User{DisplayName: "New display name", ID: "unknown-user"}, nil)
			},
			ExpectedResult: "new-user-id",
		},
		{
			Name:        "Uknown user with display name not maching an already existing synthetic user",
			UserID:      "unknown-user",
			DisplayName: "Unknown User",
			SetupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", "unknown-user").Return("", nil).Times(1)
				store.On("SetUserInfo", "new-user-id", "unknown-user", mock.Anything).Return(nil).Times(1)
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByEmail", "unknown-user@msteamssync").Return(nil, model.NewAppError("test", "not-found", nil, "", http.StatusNotFound)).Times(1)
				api.On("CreateUser", mock.MatchedBy(func(user *model.User) bool {
					if user.Username != "unknown-user-unknown-user" {
						return false
					}
					if user.FirstName != "Unknown User" {
						return false
					}
					if user.Email != "unknown-user@msteamssync" {
						return false
					}
					return true
				})).Return(&model.User{Id: "new-user-id"}, nil).Times(1)
			},
			SetupAppClient: func(*mocks.Client) {},
			ExpectedResult: "new-user-id",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			ah := newTestHandler()
			test.SetupAPI(ah.plugin.GetAPI().(*plugintest.API))
			test.SetupStore(ah.plugin.GetStore().(*storemocks.Store))
			test.SetupAppClient(ah.plugin.GetClientForApp().(*mocks.Client))
			result, err := ah.getOrCreateSyntheticUser(test.UserID, test.DisplayName)
			assert.Equal(test.ExpectedResult, result)
			if test.ExpectedError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}
