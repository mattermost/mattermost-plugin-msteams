package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestSubscriptionValidation(t *testing.T) {
	plugin := newTestPlugin(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/changes?validationToken=test", nil)

	plugin.ServeHTTP(nil, w, r)

	result := w.Result()
	assert.NotNil(t, result)
	defer result.Body.Close()
	bodyBytes, err := io.ReadAll(result.Body)
	assert.Nil(t, err)
	bodyString := string(bodyBytes)

	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, "test", bodyString)
}

func TestSubscriptionInvalidRequest(t *testing.T) {
	plugin := newTestPlugin(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/changes", strings.NewReader(""))

	plugin.ServeHTTP(nil, w, r)

	result := w.Result()
	assert.NotNil(t, result)
	defer result.Body.Close()
	bodyBytes, err := io.ReadAll(result.Body)
	assert.Nil(t, err)
	bodyString := string(bodyBytes)

	assert.Equal(t, 400, result.StatusCode)
	assert.Equal(t, "unable to get the activities from the message\n", bodyString)
}

func TestSubscriptionNewMesage(t *testing.T) {
	plugin := newTestPlugin(t)
	ttcases := []struct {
		Name          string
		Activities    Activities
		PopulateMocks func()
		ExpectedCode  int
		ExpectedBody  string
	}{
		{
			"Valid message",
			Activities{
				Value: []msteams.Activity{
					{
						Resource:                       "teams('team-id')/channels('channel-id')/messages('message-id')",
						ChangeType:                     "created",
						ClientState:                    "webhooksecret",
						SubscriptionExpirationDateTime: time.Now().Add(10 * time.Minute),
					},
				},
			},
			func() {
			},
			202,
			"",
		},
		{
			"Valid reply",
			Activities{
				Value: []msteams.Activity{
					{
						Resource:                       "teams('team-id')/channels('channel-id')/messages('message-id')/replies('reply-id')",
						ChangeType:                     "created",
						ClientState:                    "webhooksecret",
						SubscriptionExpirationDateTime: time.Now().Add(10 * time.Minute),
					},
				},
			},
			func() {
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&oauth2.Token{}, nil)
			},
			202,
			"",
		},
		{
			"Message not found",
			Activities{
				Value: []msteams.Activity{
					{
						Resource:                       "teams('team-id')/channels('channel-id')/messages('message-id')",
						ChangeType:                     "created",
						ClientState:                    "webhooksecret",
						SubscriptionExpirationDateTime: time.Now().Add(10 * time.Minute),
					},
				},
			},
			func() {
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&oauth2.Token{}, nil)
			},
			202,
			"",
		},
		{
			"Invalid activity",
			Activities{
				Value: []msteams.Activity{
					{
						Resource:                       "test",
						ChangeType:                     "created",
						ClientState:                    "webhooksecret",
						SubscriptionExpirationDateTime: time.Now().Add(10 * time.Minute),
					},
				},
			},
			func() {
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&oauth2.Token{}, nil)
			},
			202,
			"",
		},
		{
			"Invalid webhook secret",
			Activities{
				Value: []msteams.Activity{
					{
						Resource:                       "teams('team-id')/channels('channel-id')/messages('message-id')",
						ChangeType:                     "created",
						ClientState:                    "invalid",
						SubscriptionExpirationDateTime: time.Now().Add(10 * time.Minute),
					},
				},
			},
			func() {
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&oauth2.Token{}, nil)
				plugin.API.(*plugintest.API).On("LogError", "Unable to process created activity", "activity", mock.Anything, "error", "Invalid webhook secret").Return(nil)
			},
			400,
			"Invalid webhook secret\n",
		},
	}
	for _, tc := range ttcases {
		t.Run(tc.Name, func(t *testing.T) {
			data, err := json.Marshal(tc.Activities)
			require.NoError(t, err)

			tc.PopulateMocks()

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/changes", bytes.NewReader(data))

			plugin.ServeHTTP(nil, w, r)

			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()
			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(t, err)
			bodyString := string(bodyBytes)

			assert.Equal(t, tc.ExpectedCode, result.StatusCode)
			assert.Equal(t, tc.ExpectedBody, bodyString)
		})
	}
}

func TestGetAvatarFromCache(t *testing.T) {
	plugin := newTestPlugin(t)

	plugin.store.(*storemocks.Store).On("GetAvatarCache", "user-id").Return([]byte("fake-avatar"), nil).Times(1)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/avatar/user-id", nil)

	plugin.ServeHTTP(nil, w, r)

	result := w.Result()
	assert.NotNil(t, result)
	defer result.Body.Close()
	bodyBytes, err := io.ReadAll(result.Body)
	assert.Nil(t, err)
	bodyString := string(bodyBytes)

	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, "fake-avatar", bodyString)
}

func TestGetAvatarFromServer(t *testing.T) {
	plugin := newTestPlugin(t)

	plugin.store.(*storemocks.Store).On("GetAvatarCache", "user-id").Return(nil, &model.AppError{Message: "not-found"}).Times(1)
	plugin.msteamsAppClient.(*clientmocks.Client).On("GetUserAvatar", "user-id").Return([]byte("fake-avatar"), nil).Times(1)
	plugin.store.(*storemocks.Store).On("SetAvatarCache", "user-id", []byte("fake-avatar")).Return(nil).Times(1)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/avatar/user-id", nil)

	plugin.ServeHTTP(nil, w, r)

	result := w.Result()
	assert.NotNil(t, result)
	defer result.Body.Close()
	bodyBytes, err := io.ReadAll(result.Body)
	assert.Nil(t, err)
	bodyString := string(bodyBytes)

	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, "fake-avatar", bodyString)
}

func TestGetAvatarNotFound(t *testing.T) {
	plugin := newTestPlugin(t)

	plugin.store.(*storemocks.Store).On("GetAvatarCache", "user-id").Return(nil, &model.AppError{Message: "not-found"}).Times(1)
	plugin.msteamsAppClient.(*clientmocks.Client).On("GetUserAvatar", "user-id").Return(nil, errors.New("not-found")).Times(1)
	plugin.API.(*plugintest.API).On("LogError", "Unable to read avatar", "error", "not-found").Return(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/avatar/user-id", nil)

	plugin.ServeHTTP(nil, w, r)

	result := w.Result()
	assert.NotNil(t, result)
	defer result.Body.Close()
	bodyBytes, err := io.ReadAll(result.Body)
	assert.Nil(t, err)
	bodyString := string(bodyBytes)

	assert.Equal(t, 404, result.StatusCode)
	assert.Equal(t, "avatar not found\n", bodyString)
}

func TestProcessActivity(t *testing.T) {
	newTime := time.Now().Add(30 * time.Minute)
	for _, test := range []struct {
		Name               string
		SetupAPI           func(*plugintest.API)
		SetupClient        func(client *clientmocks.Client, uclient *clientmocks.Client)
		SetupStore         func(*storemocks.Store)
		RequestBody        string
		ValidationToken    string
		ExpectedStatusCode int
		ExpectedResult     string
	}{
		{
			Name:               "ProcessActivity: With validation token present",
			SetupAPI:           func(api *plugintest.API) {},
			SetupClient:        func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupStore:         func(store *storemocks.Store) {},
			ValidationToken:    "mockValidationToken",
			ExpectedStatusCode: http.StatusOK,
			ExpectedResult:     "mockValidationToken",
		},
		{
			Name:               "ProcessActivity: Invalid body",
			SetupAPI:           func(api *plugintest.API) {},
			SetupClient:        func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupStore:         func(store *storemocks.Store) {},
			RequestBody:        `{`,
			ExpectedStatusCode: http.StatusBadRequest,
			ExpectedResult:     "unable to get the activities from the message\n",
		},
		{
			Name: "ProcessActivity: Valid body with invalid webhook secret",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to process created activity", "activity", mock.Anything, "error", mock.Anything).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupStore:  func(store *storemocks.Store) {},
			RequestBody: `{
				"Value": [{
				"Resource": "mockResource",
				"ClientState": "mockClientState",
				"ChangeType": "mockChangeType",
				"LifecycleEvent": "mockLifecycleEvent"
			}]}`,
			ExpectedStatusCode: http.StatusBadRequest,
			ExpectedResult:     "Invalid webhook secret\n",
		},
		{
			Name:     "ProcessActivity: Valid body with valid webhook secret",
			SetupAPI: func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("RefreshSubscription", "mockID").Return(&newTime, nil)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("UpdateSubscriptionExpiresOn", "mockID", newTime).Return(nil)
			},
			RequestBody: `{
				"Value": [{
				"SubscriptionID": "mockID",
				"Resource": "mockResource",
				"ClientState": "webhooksecret",
				"ChangeType": "mockChangeType",
				"LifecycleEvent": "mockLifecycleEvent"
			}]}`,
			ExpectedStatusCode: http.StatusAccepted,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			test.SetupStore(plugin.store.(*storemocks.Store))
			test.SetupAPI(plugin.API.(*plugintest.API))
			test.SetupClient(plugin.msteamsAppClient.(*clientmocks.Client), plugin.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/changes", bytes.NewBufferString(test.RequestBody))
			if test.ValidationToken != "" {
				queryParams := url.Values{
					"validationToken": {"mockValidationToken"},
				}

				r.URL.RawQuery = queryParams.Encode()
			}
			plugin.ServeHTTP(nil, w, r)
			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()
			bodyBytes, _ := io.ReadAll(result.Body)
			bodyString := string(bodyBytes)
			assert.Equal(test.ExpectedStatusCode, result.StatusCode)
			assert.Equal(test.ExpectedResult, bodyString)
		})
	}
}

func TestProcessLifecycle(t *testing.T) {
	newTime := time.Now().Add(30 * time.Minute)
	for _, test := range []struct {
		Name               string
		ValidationToken    string
		SetupAPI           func(*plugintest.API)
		SetupClient        func(client *clientmocks.Client, uclient *clientmocks.Client)
		SetupStore         func(*storemocks.Store)
		RequestBody        string
		ExpectedStatusCode int
		ExpectedResult     string
	}{
		{
			Name:            "ProcessLifecycle: With validation token present",
			SetupAPI:        func(api *plugintest.API) {},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupStore:      func(store *storemocks.Store) {},
			ValidationToken: "mockValidationToken",
			RequestBody: `{
				"Value": [{
				"Resource": "mockResource",
				"ClientState": "webhooksecret",
				"ChangeType": "mockChangeType",
				"LifecycleEvent": "mockLifecycleEvent"
			}]}`,
			ExpectedStatusCode: http.StatusOK,
			ExpectedResult:     "mockValidationToken",
		},
		{
			Name:               "ProcessLifecycle: Invalid body",
			SetupAPI:           func(api *plugintest.API) {},
			SetupClient:        func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupStore:         func(store *storemocks.Store) {},
			RequestBody:        `{`,
			ExpectedStatusCode: http.StatusBadRequest,
			ExpectedResult:     "unable to get the lifecycle events from the message\n",
		},
		{
			Name: "ProcessLifecycle: Valid body with invalid webhook secret",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Invalid webhook secret recevied in lifecycle event").Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupStore:  func(store *storemocks.Store) {},
			RequestBody: `{
				"Value": [{
				"Resource": "mockResource",
				"ClientState": "mockClientState",
				"ChangeType": "mockChangeType",
				"LifecycleEvent": "mockLifecycleEvent"
			}]}`,
			ExpectedStatusCode: http.StatusOK,
		},
		{
			Name:        "ProcessLifecycle: Valid body with valid webhook secret and without refresh needed",
			SetupAPI:    func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetSubscriptionType", "mockID").Return("allChats", nil)
			},
			RequestBody: `{
				"Value": [{
				"SubscriptionID": "mockID",
				"Resource": "mockResource",
				"ClientState": "webhooksecret",
				"ChangeType": "mockChangeType",
				"LifecycleEvent": "mockLifecycleEvent"
			}]}`,
			ExpectedStatusCode: http.StatusOK,
		},
		{
			Name:     "ProcessLifecycle: Valid body with valid webhook secret and with refresh needed",
			SetupAPI: func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("RefreshSubscription", "mockID").Return(&newTime, nil)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetSubscriptionType", "mockID").Return("allChats", nil)
				store.On("UpdateSubscriptionExpiresOn", "mockID", newTime).Return(nil)
			},
			RequestBody: `{
				"Value": [{
				"SubscriptionID": "mockID",
				"ClientState": "webhooksecret",
				"ChangeType": "mockChangeType",
				"LifecycleEvent": "reauthorizationRequired"
			}]}`,
			ExpectedStatusCode: http.StatusOK,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			test.SetupStore(plugin.store.(*storemocks.Store))
			test.SetupAPI(plugin.API.(*plugintest.API))
			test.SetupClient(plugin.msteamsAppClient.(*clientmocks.Client), plugin.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/lifecycle", bytes.NewBufferString(test.RequestBody))
			if test.ValidationToken != "" {
				queryParams := url.Values{
					"validationToken": {"mockValidationToken"},
				}

				r.URL.RawQuery = queryParams.Encode()
			}
			plugin.ServeHTTP(nil, w, r)
			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()
			bodyBytes, _ := io.ReadAll(result.Body)
			bodyString := string(bodyBytes)
			assert.Equal(test.ExpectedStatusCode, result.StatusCode)
			assert.Equal(test.ExpectedResult, bodyString)
		})
	}
}

func TestAutocompleteTeams(t *testing.T) {
	for _, test := range []struct {
		Name           string
		SetupAPI       func(*plugintest.API)
		SetupStore     func(*storemocks.Store)
		SetupClient    func(*clientmocks.Client, *clientmocks.Client)
		ExpectedResult []model.AutocompleteListItem
	}{
		{
			Name: "AutocompleteTeams: Unable to get client for the user",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get the client for user", "Error", "not connected user").Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:    func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedResult: []model.AutocompleteListItem{},
		},
		{
			Name: "AutocompleteTeams: Unable to get the teams list",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get the MS Teams teams", "Error", "unable to get the teams list").Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("ListTeams").Return(nil, errors.New("unable to get the teams list")).Times(1)
			},
			ExpectedResult: []model.AutocompleteListItem{},
		},
		{
			Name: "AutocompleteTeams: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogDebug", "Successfully fetched the list of teams", "Count", 2).Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("ListTeams").Return([]msteams.Team{
					{
						ID:          "mockTeamsTeamID-1",
						DisplayName: "mockDisplayName-1",
						Description: "mockDescription-1",
					},
					{
						ID:          "mockTeamsTeamID-2",
						DisplayName: "mockDisplayName-2",
						Description: "mockDescription-2",
					},
				}, nil).Times(1)
			},
			ExpectedResult: []model.AutocompleteListItem{
				{
					Item:     "mockTeamsTeamID-1",
					Hint:     "mockDisplayName-1",
					HelpText: "mockDescription-1",
				},
				{
					Item:     "mockTeamsTeamID-2",
					Hint:     "mockDisplayName-2",
					HelpText: "mockDescription-2",
				},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			test.SetupAPI(plugin.API.(*plugintest.API))
			test.SetupStore(plugin.store.(*storemocks.Store))
			test.SetupClient(plugin.msteamsAppClient.(*clientmocks.Client), plugin.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/autocomplete/teams", nil)
			r.Header.Add("Mattermost-User-ID", testutils.GetID())
			plugin.ServeHTTP(nil, w, r)
			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()

			var list []model.AutocompleteListItem
			err := json.NewDecoder(result.Body).Decode(&list)
			require.Nil(t, err)
			assert.Equal(test.ExpectedResult, list)
		})
	}
}

func TestAutocompleteChannels(t *testing.T) {
	for _, test := range []struct {
		Name           string
		QueryParams    string
		SetupAPI       func(*plugintest.API)
		SetupStore     func(*storemocks.Store)
		SetupClient    func(*clientmocks.Client, *clientmocks.Client)
		ExpectedResult []model.AutocompleteListItem
	}{
		{
			Name:           "AutocompleteChannels: Query params not present",
			SetupAPI:       func(a *plugintest.API) {},
			SetupStore:     func(store *storemocks.Store) {},
			SetupClient:    func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedResult: []model.AutocompleteListItem{},
		},
		{
			Name: "AutocompleteChannels: Unable to get client for the user",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get the client for user", "Error", "not connected user").Once()
			},
			QueryParams: "mockData-1 mockData-2 mockData-3",
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:    func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			ExpectedResult: []model.AutocompleteListItem{},
		},
		{
			Name: "AutocompleteChannels: Unable to get the channels list",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get the channels for MS Teams team", "TeamID", "mockData-3", "Error", "unable to get the channels list").Once()
			},
			QueryParams: "mockData-1 mockData-2 mockData-3",
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("ListChannels", "mockData-3").Return(nil, errors.New("unable to get the channels list")).Times(1)
			},
			ExpectedResult: []model.AutocompleteListItem{},
		},
		{
			Name: "AutocompleteChannels: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogDebug", "Successfully fetched the list of channels for MS Teams team", "TeamID", "mockData-3", "Count", 2).Once()
			},
			QueryParams: "mockData-1 mockData-2 mockData-3",
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("ListChannels", "mockData-3").Return([]msteams.Channel{
					{
						ID:          "mockTeamsTeamID-1",
						DisplayName: "mockDisplayName-1",
						Description: "mockDescription-1",
					},
					{
						ID:          "mockTeamsTeamID-2",
						DisplayName: "mockDisplayName-2",
						Description: "mockDescription-2",
					},
				}, nil).Times(1)
			},
			ExpectedResult: []model.AutocompleteListItem{
				{
					Item:     "mockTeamsTeamID-1",
					Hint:     "mockDisplayName-1",
					HelpText: "mockDescription-1",
				},
				{
					Item:     "mockTeamsTeamID-2",
					Hint:     "mockDisplayName-2",
					HelpText: "mockDescription-2",
				},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			test.SetupAPI(plugin.API.(*plugintest.API))
			test.SetupStore(plugin.store.(*storemocks.Store))
			test.SetupClient(plugin.msteamsAppClient.(*clientmocks.Client), plugin.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/autocomplete/channels", nil)
			if test.QueryParams != "" {
				queryParams := url.Values{
					"parsed": {test.QueryParams},
				}

				r.URL.RawQuery = queryParams.Encode()
			}

			r.Header.Add("Mattermost-User-ID", testutils.GetID())
			plugin.ServeHTTP(nil, w, r)
			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()

			var list []model.AutocompleteListItem
			err := json.NewDecoder(result.Body).Decode(&list)
			require.Nil(t, err)
			assert.Equal(test.ExpectedResult, list)
		})
	}
}

func TestNeedsConnect(t *testing.T) {
	for _, test := range []struct {
		Name                  string
		SetupPlugin           func(*plugintest.API)
		SetupStore            func(*storemocks.Store)
		EnforceConnectedUsers bool
		EnabledTeams          string
		ExpectedResult        string
	}{
		{
			Name:           "NeedsConnect: EnforceConnectedUsers is false",
			SetupPlugin:    func(api *plugintest.API) {},
			SetupStore:     func(store *storemocks.Store) {},
			ExpectedResult: "{\"canSkip\":false,\"needsConnect\":false}",
		},
		{
			Name:        "NeedsConnect: Unable to get the client",
			SetupPlugin: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			EnforceConnectedUsers: true,
			ExpectedResult:        "{\"canSkip\":false,\"needsConnect\":true}",
		},
		{
			Name: "NeedsConnect: Enabled teams is non empty and not matches with the team",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetTeamsForUser", testutils.GetID()).Return([]*model.Team{
					{
						Id: "mockTeam",
					},
				}, nil)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			EnforceConnectedUsers: true,
			EnabledTeams:          "mockTeamID",
			ExpectedResult:        "{\"canSkip\":false,\"needsConnect\":false}",
		},
		{
			Name: "NeedsConnect: Enabled teams is non empty and matches with the team",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetTeamsForUser", testutils.GetID()).Return([]*model.Team{
					{
						Id: "mockTeamID",
					},
				}, nil)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			EnforceConnectedUsers: true,
			EnabledTeams:          "mockTeamID",
			ExpectedResult:        "{\"canSkip\":false,\"needsConnect\":true}",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			plugin.configuration.EnforceConnectedUsers = test.EnforceConnectedUsers
			plugin.configuration.EnabledTeams = test.EnabledTeams
			test.SetupPlugin(plugin.API.(*plugintest.API))
			test.SetupStore(plugin.store.(*storemocks.Store))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/needsConnect", nil)
			r.Header.Add("Mattermost-User-ID", testutils.GetID())
			plugin.ServeHTTP(nil, w, r)
			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()

			bodyBytes, _ := io.ReadAll(result.Body)
			bodyString := string(bodyBytes)
			assert.Equal(test.ExpectedResult, bodyString)
		})
	}
}

func TestDisconnect(t *testing.T) {
	for _, test := range []struct {
		Name                  string
		SetupPlugin           func(*plugintest.API)
		SetupStore            func(*storemocks.Store)
		EnforceConnectedUsers bool
		EnabledTeams          string
		ExpectedResult        string
		ExpectedStatusCode    int
	}{
		{
			Name:        "Disconnect: user successfully disconnected",
			SetupPlugin: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetUserID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, nil).Times(1)
				store.On("SetUserInfo", testutils.GetUserID(), testutils.GetID(), (*oauth2.Token)(nil)).Return(nil).Times(1)
			},
			ExpectedResult:     "\"Your account has been disconnected.\"",
			ExpectedStatusCode: http.StatusOK,
		},
		{
			Name: "Disconnect: could not find Teams ID",
			SetupPlugin: func(api *plugintest.API) {
				api.On("LogError", "The account is not connected.", "UserID", testutils.GetUserID()).Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetUserID()).Return(testutils.GetID(), errors.New("could not find Teams ID")).Times(1)
			},
			ExpectedResult:     "The account is not connected.\n",
			ExpectedStatusCode: http.StatusBadRequest,
		},
		{
			Name: "Disconnect: could not get token for MM user",
			SetupPlugin: func(api *plugintest.API) {
				api.On("LogError", "The account is not connected.", "UserID", testutils.GetUserID()).Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetUserID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, errors.New("could not get token for MM user")).Times(1)
			},
			ExpectedResult:     "The account is not connected.\n",
			ExpectedStatusCode: http.StatusBadRequest,
		},
		{
			Name: "Disconnect: error occurred while setting the user info",
			SetupPlugin: func(api *plugintest.API) {
				api.On("LogError", "Error occurred while disconnecting the user.", "UserID", testutils.GetUserID(), "Error", "error occurred while setting the user info").Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetUserID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, nil).Times(1)
				store.On("SetUserInfo", testutils.GetUserID(), testutils.GetID(), (*oauth2.Token)(nil)).Return(errors.New("error occurred while setting the user info")).Times(1)
			},
			ExpectedResult:     "Error occurred while disconnecting the user.\n",
			ExpectedStatusCode: http.StatusInternalServerError,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			plugin.configuration.EnforceConnectedUsers = test.EnforceConnectedUsers
			plugin.configuration.EnabledTeams = test.EnabledTeams
			test.SetupPlugin(plugin.API.(*plugintest.API))
			test.SetupStore(plugin.store.(*storemocks.Store))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/disconnect", nil)
			r.Header.Add("Mattermost-User-ID", testutils.GetUserID())
			plugin.ServeHTTP(nil, w, r)
			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()

			bodyBytes, _ := io.ReadAll(result.Body)
			bodyString := string(bodyBytes)
			assert.Equal(test.ExpectedResult, bodyString)
			assert.Equal(result.StatusCode, test.ExpectedStatusCode)
		})
	}
}
