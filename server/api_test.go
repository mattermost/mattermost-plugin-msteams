package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	metricsmocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
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
	plugin.metricsService.(*metricsmocks.Metrics).On("ObserveAPIEndpointDuration", "/changes", http.MethodPost, fmt.Sprint(http.StatusBadRequest), mock.AnythingOfType("float64")).Times(1)
	plugin.metricsService.(*metricsmocks.Metrics).On("IncrementHTTPErrors").Times(1)

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
				plugin.metricsService.(*metricsmocks.Metrics).On("ObserveChangeEventTotal", metrics.ActionCreated).Times(1)
			},
			http.StatusAccepted,
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
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&oauth2.Token{}, nil).Times(1)
				plugin.metricsService.(*metricsmocks.Metrics).On("ObserveChangeEventTotal", metrics.ActionCreated).Times(1)
			},
			http.StatusAccepted,
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
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&oauth2.Token{}, nil).Times(1)
				plugin.metricsService.(*metricsmocks.Metrics).On("ObserveChangeEventTotal", metrics.ActionCreated).Times(1)
			},
			http.StatusAccepted,
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
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&oauth2.Token{}, nil).Times(1)
				plugin.metricsService.(*metricsmocks.Metrics).On("ObserveChangeEventTotal", metrics.ActionCreated).Times(1)
			},
			http.StatusAccepted,
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
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&oauth2.Token{}, nil).Times(1)
				plugin.API.(*plugintest.API).On("LogError", "Unable to process created activity", "activity", mock.Anything, "error", "Invalid webhook secret").Return(nil).Times(1)
			},
			http.StatusBadRequest,
			"Invalid webhook secret\n",
		},
		{
			"Encrypted message on encrypted subscription",
			Activities{
				Value: []msteams.Activity{
					{
						Resource:                       "teams('team-id')/channels('channel-id')/messages('message-id')/replies('reply-id')",
						ChangeType:                     "created",
						ClientState:                    "webhooksecret",
						SubscriptionExpirationDateTime: time.Now().Add(10 * time.Minute),
						EncryptedContent:               &msteams.EncryptedContent{},
					},
				},
			},
			func() {
				plugin.configuration.CertificateKey = "test"
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&oauth2.Token{}, nil)
				plugin.API.(*plugintest.API).On("LogError", "Invalid encrypted content", "error", "invalid certificate key").Return(nil)
			},
			http.StatusBadRequest,
			"invalid certificate key\n\n",
		},
		{
			"Non encrypted message on encrypted subscription",
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
				plugin.configuration.CertificateKey = "test"
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&oauth2.Token{}, nil)
			},
			http.StatusBadRequest,
			"Not encrypted content for encrypted subscription\n",
		},
	}
	for _, tc := range ttcases {
		t.Run(tc.Name, func(t *testing.T) {
			data, err := json.Marshal(tc.Activities)
			require.NoError(t, err)

			tc.PopulateMocks()
			if tc.ExpectedBody != "" {
				plugin.metricsService.(*metricsmocks.Metrics).On("IncrementHTTPErrors").Times(1)
			} else {
				plugin.metricsService.(*metricsmocks.Metrics).On("IncrementChangeEventQueueLength", metrics.ActionCreated).Times(1)
			}

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
	plugin.API.(*plugintest.API).On("LogError", "Unable to get user avatar", "msteamsUserID", "user-id", "error", "not-found").Return(nil)
	plugin.metricsService.(*metricsmocks.Metrics).On("IncrementHTTPErrors").Times(1)

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
			if test.ExpectedResult != "" {
				plugin.metricsService.(*metricsmocks.Metrics).On("IncrementHTTPErrors").Times(1)
			}

			if test.ExpectedStatusCode == http.StatusAccepted {
				plugin.metricsService.(*metricsmocks.Metrics).On("ObserveChangeEventTotal", "mockChangeType").Times(1)
				plugin.metricsService.(*metricsmocks.Metrics).On("IncrementChangeEventQueueLength", "mockChangeType").Times(1)
			}

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
		SetupMetrics       func(mockmetrics *metricsmocks.Metrics)
		RequestBody        string
		ExpectedStatusCode int
		ExpectedResult     string
	}{
		{
			Name:        "ProcessLifecycle: With validation token present",
			SetupAPI:    func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupStore:  func(store *storemocks.Store) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveLifecycleEvent", "mockLifecycleEvent", "").Times(1)
			},
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
			Name:        "ProcessLifecycle: Invalid body",
			SetupAPI:    func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupStore:  func(store *storemocks.Store) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			RequestBody:        `{`,
			ExpectedStatusCode: http.StatusBadRequest,
			ExpectedResult:     "unable to get the lifecycle events from the message\n",
		},
		{
			Name: "ProcessLifecycle: Valid body with invalid webhook secret",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Invalid webhook secret received in lifecycle event").Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupStore:  func(store *storemocks.Store) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
				mockmetrics.On("ObserveLifecycleEvent", "mockLifecycleEvent", mock.AnythingOfType("string")).Times(1)
			},

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
			Name:        "ProcessLifecycle: Valid body with valid webhook secret and without refresh needed",
			SetupAPI:    func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetChannelSubscription", "mockID").Return(&storemodels.ChannelSubscription{
					TeamID:    testutils.GetTeamsTeamID(),
					ChannelID: testutils.GetMSTeamsChannelID(),
				}, nil).Once()
				store.On("GetLinkByMSTeamsChannelID", testutils.GetTeamsTeamID(), testutils.GetMSTeamsChannelID()).Return(nil, nil).Once()
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveLifecycleEvent", "mockLifecycleEvent", "").Times(1)
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
				store.On("GetChannelSubscription", "mockID").Return(&storemodels.ChannelSubscription{
					TeamID:    testutils.GetTeamsTeamID(),
					ChannelID: testutils.GetMSTeamsChannelID(),
				}, nil).Once()
				store.On("GetLinkByMSTeamsChannelID", testutils.GetTeamsTeamID(), testutils.GetMSTeamsChannelID()).Return(nil, nil).Once()
				store.On("UpdateSubscriptionExpiresOn", "mockID", newTime).Return(nil)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionRefreshed).Times(1)
				mockmetrics.On("ObserveLifecycleEvent", "reauthorizationRequired", "").Times(1)
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
			test.SetupMetrics(plugin.metricsService.(*metricsmocks.Metrics))
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
		SetupMetrics   func(metrics *metricsmocks.Metrics)
		ExpectedResult []model.AutocompleteListItem
	}{
		{
			Name: "AutocompleteTeams: Unable to get client for the user",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get the client for user", "MMUserID", testutils.GetID(), "Error", "not connected user").Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:    func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:   func(metrics *metricsmocks.Metrics) {},
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
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.ListTeams", "false", mock.AnythingOfType("float64")).Once()
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
				uclient.On("ListTeams").Return([]*clientmodels.Team{
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
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.ListTeams", "true", mock.AnythingOfType("float64")).Once()
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
			test.SetupClient(plugin.msteamsAppClient.(*clientmocks.Client), plugin.clientBuilderWithToken("", "", "",
				"", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(plugin.metricsService.(*metricsmocks.Metrics))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/autocomplete/teams", nil)
			r.Header.Add(HeaderMattermostUserID, testutils.GetID())
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
		SetupMetrics   func(metrics *metricsmocks.Metrics)
		ExpectedResult []model.AutocompleteListItem
	}{
		{
			Name:           "AutocompleteChannels: Query params not present",
			SetupAPI:       func(a *plugintest.API) {},
			SetupStore:     func(store *storemocks.Store) {},
			SetupClient:    func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:   func(metrics *metricsmocks.Metrics) {},
			ExpectedResult: []model.AutocompleteListItem{},
		},
		{
			Name: "AutocompleteChannels: Unable to get client for the user",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to get the client for user", "MMUserID", testutils.GetID(), "Error", "not connected user").Once()
			},
			QueryParams: "mockData-1 mockData-2 mockData-3",
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:    func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:   func(metrics *metricsmocks.Metrics) {},
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
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.ListChannels", "false", mock.AnythingOfType("float64")).Once()
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
				uclient.On("ListChannels", "mockData-3").Return([]*clientmodels.Channel{
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
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.ListChannels", "true", mock.AnythingOfType("float64")).Once()
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
			test.SetupMetrics(plugin.metricsService.(*metricsmocks.Metrics))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/autocomplete/channels", nil)
			if test.QueryParams != "" {
				queryParams := url.Values{
					"parsed": {test.QueryParams},
				}

				r.URL.RawQuery = queryParams.Encode()
			}

			r.Header.Add(HeaderMattermostUserID, testutils.GetID())
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
		SetupClient           func(*clientmocks.Client)
		SetupStore            func(store *storemocks.Store)
		SetupMetrics          func(metrics *metricsmocks.Metrics)
		EnforceConnectedUsers bool
		EnabledTeams          string
		ExpectedResult        string
	}{
		{
			Name:        "NeedsConnect: EnforceConnectedUsers is false and user is connected",
			SetupPlugin: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetMe").Return(&clientmodels.User{
					DisplayName: "mockUser",
					ID:          "mockUserID",
				}, nil)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetMe", "true", mock.AnythingOfType("float64")).Once()
			},
			ExpectedResult: "{\"canSkip\":false,\"connected\":true,\"msteamsUserId\":\"mockUserID\",\"needsConnect\":false,\"username\":\"mockUser\"}",
		},
		{
			Name: "NeedsConnect: EnforceConnectedUsers is false and user is connected but unable to get the user",
			SetupPlugin: func(api *plugintest.API) {
				api.On("LogError", "Unable to get MS Teams user", "error", "unable to get the user").Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetMe").Return(nil, errors.New("unable to get the user")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetMe", "false", mock.AnythingOfType("float64")).Once()
			},
			ExpectedResult: "{\"canSkip\":false,\"connected\":true,\"msteamsUserId\":\"\",\"needsConnect\":false,\"username\":\"\"}",
		},
		{
			Name: "NeedsConnect: Unable to get the client",
			SetupPlugin: func(api *plugintest.API) {
				api.On("LogError", "Unable to get client for user", "error", "not connected user").Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:           func(client *clientmocks.Client) {},
			SetupMetrics:          func(metrics *metricsmocks.Metrics) {},
			EnforceConnectedUsers: true,
			ExpectedResult:        "{\"canSkip\":false,\"connected\":false,\"msteamsUserId\":\"\",\"needsConnect\":true,\"username\":\"\"}",
		},
		{
			Name: "NeedsConnect: Enabled teams is non empty and not matches with the team",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetTeamsForUser", testutils.GetID()).Return([]*model.Team{
					{
						Id: "mockTeam",
					},
				}, nil)
				api.On("LogError", "Unable to get client for user", "error", "not connected user").Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:           func(client *clientmocks.Client) {},
			SetupMetrics:          func(metrics *metricsmocks.Metrics) {},
			EnforceConnectedUsers: true,
			EnabledTeams:          "mockTeamID",
			ExpectedResult:        "{\"canSkip\":false,\"connected\":false,\"msteamsUserId\":\"\",\"needsConnect\":false,\"username\":\"\"}",
		},
		{
			Name: "NeedsConnect: Enabled teams is non empty and matches with the team",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetTeamsForUser", testutils.GetID()).Return([]*model.Team{
					{
						Id: "mockTeamID",
					},
				}, nil)
				api.On("LogError", "Unable to get client for user", "error", "not connected user").Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:           func(client *clientmocks.Client) {},
			SetupMetrics:          func(metrics *metricsmocks.Metrics) {},
			EnforceConnectedUsers: true,
			EnabledTeams:          "mockTeamID",
			ExpectedResult:        "{\"canSkip\":false,\"connected\":false,\"msteamsUserId\":\"\",\"needsConnect\":true,\"username\":\"\"}",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			plugin.configuration.EnforceConnectedUsers = test.EnforceConnectedUsers
			plugin.configuration.EnabledTeams = test.EnabledTeams
			test.SetupPlugin(plugin.API.(*plugintest.API))
			test.SetupStore(plugin.store.(*storemocks.Store))
			test.SetupClient(plugin.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(plugin.metricsService.(*metricsmocks.Metrics))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/needsConnect", nil)
			r.Header.Add(HeaderMattermostUserID, testutils.GetID())
			plugin.ServeHTTP(nil, w, r)
			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()

			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(err)

			bodyString := string(bodyBytes)
			assert.Equal(test.ExpectedResult, bodyString)
		})
	}
}

func TestConnect(t *testing.T) {
	for _, test := range []struct {
		Name               string
		SetupPlugin        func(*plugintest.API)
		SetupStore         func(*storemocks.Store)
		ExpectedResult     string
		ExpectedStatusCode int
	}{
		{
			Name: "connect: User connected",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(1)
				api.On("KVSet", fmt.Sprintf("_code_verifier_%s", testutils.GetUserID()), mock.AnythingOfType("[]uint8")).Return(nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(nil).Times(1)
			},
			ExpectedStatusCode: http.StatusOK,
		},
		{
			Name: "connect: Error in storing the OAuth state",
			SetupPlugin: func(api *plugintest.API) {
				api.On("LogError", "Error in storing the OAuth state", "error", "error in storing the oauth state").Return(nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(errors.New("error in storing the oauth state")).Times(1)
			},
			ExpectedResult:     "Error in trying to connect the account, please try again.\n",
			ExpectedStatusCode: http.StatusInternalServerError,
		},
		{
			Name: "connect: Error in storing the code verifier",
			SetupPlugin: func(api *plugintest.API) {
				api.On("LogError", "Error in storing the code verifier", "error", "error in storing the code verifier").Return(nil).Times(1)
				api.On("KVSet", mock.AnythingOfType("string"), mock.AnythingOfType("[]uint8")).Return(&model.AppError{
					Message: "error in storing the code verifier",
				}).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("StoreOAuth2State", mock.AnythingOfType("string")).Return(nil).Times(1)
			},
			ExpectedResult:     "Error in trying to connect the account, please try again.\n",
			ExpectedStatusCode: http.StatusInternalServerError,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			if test.ExpectedResult != "" {
				plugin.metricsService.(*metricsmocks.Metrics).On("IncrementHTTPErrors").Times(1)
			}

			mockAPI := &plugintest.API{}

			plugin.SetAPI(mockAPI)
			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(mockAPI)
			test.SetupStore(plugin.store.(*storemocks.Store))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/connect", nil)
			r.Header.Add(HeaderMattermostUserID, testutils.GetUserID())
			plugin.ServeHTTP(nil, w, r)

			result := w.Result()
			defer result.Body.Close()

			assert.NotNil(t, result)
			assert.Equal(test.ExpectedStatusCode, result.StatusCode)

			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(err)
			if test.ExpectedResult != "" {
				assert.Equal(test.ExpectedResult, string(bodyBytes))
			}
		})
	}
}

func TestGetConnectedUsers(t *testing.T) {
	for _, test := range []struct {
		Name               string
		SetupPlugin        func(*plugintest.API)
		SetupStore         func(*storemocks.Store)
		ExpectedResult     string
		ExpectedStatusCode int
	}{
		{
			Name: "getConnectedUsers: Insufficient permissions for the user",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(false).Times(1)
				api.On("LogError", "Insufficient permissions", "UserID", testutils.GetUserID()).Return(nil).Times(1)
			},
			SetupStore:         func(store *storemocks.Store) {},
			ExpectedStatusCode: http.StatusForbidden,
			ExpectedResult:     "not able to authorize the user\n",
		},
		{
			Name: "getConnectedUsers: Unable to get the list of connected users from the store",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("LogError", "Unable to get connected users list", "Error", ("unable to get the list of connected users from the store")).Return(nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetConnectedUsers", 0, 100).Return(nil, errors.New("unable to get the list of connected users from the store")).Times(1)
			},
			ExpectedStatusCode: http.StatusInternalServerError,
			ExpectedResult:     "unable to get connected users list\n",
		},
		{
			Name: "getConnectedUsers: No user is connected",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetConnectedUsers", 0, 100).Return([]*storemodels.ConnectedUser{}, nil).Times(1)
			},
			ExpectedStatusCode: http.StatusOK,
		},
		{
			Name: "getConnectedUsers: Users are connected",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetConnectedUsers", 0, 100).Return([]*storemodels.ConnectedUser{
					{
						MattermostUserID: testutils.GetUserID(),
						TeamsUserID:      testutils.GetTeamsUserID(),
						Email:            testutils.GetTestEmail(),
					},
				}, nil).Times(1)
			},
			ExpectedStatusCode: http.StatusOK,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			if test.ExpectedResult != "" {
				plugin.metricsService.(*metricsmocks.Metrics).On("IncrementHTTPErrors").Times(1)
			}

			mockAPI := &plugintest.API{}

			plugin.SetAPI(mockAPI)

			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(mockAPI)
			test.SetupStore(plugin.store.(*storemocks.Store))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/connected-users", nil)
			r.Header.Add(HeaderMattermostUserID, testutils.GetUserID())
			plugin.ServeHTTP(nil, w, r)

			result := w.Result()
			defer result.Body.Close()

			assert.NotNil(t, result)
			assert.Equal(test.ExpectedStatusCode, result.StatusCode)

			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(err)
			if test.ExpectedResult != "" {
				assert.Equal(test.ExpectedResult, string(bodyBytes))
			}
		})
	}
}

func TestGetConnectedUsersFile(t *testing.T) {
	for _, test := range []struct {
		Name               string
		SetupPlugin        func(*plugintest.API)
		SetupStore         func(*storemocks.Store)
		ExpectedResult     string
		ExpectedStatusCode int
	}{
		{
			Name: "getConnectedUsers: Insufficient permissions for the user",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(false).Times(1)
				api.On("LogError", "Insufficient permissions", "UserID", testutils.GetUserID()).Return(nil).Times(1)
			},
			SetupStore:         func(store *storemocks.Store) {},
			ExpectedStatusCode: http.StatusForbidden,
			ExpectedResult:     "not able to authorize the user\n",
		},
		{
			Name: "getConnectedUsers: Unable to get the list of connected users from the store",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
				api.On("LogError", "Unable to get connected users list", "Error", ("unable to get the list of connected users from the store")).Return(nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetConnectedUsers", 0, 100).Return(nil, errors.New("unable to get the list of connected users from the store")).Times(1)
			},
			ExpectedStatusCode: http.StatusInternalServerError,
			ExpectedResult:     "unable to get connected users list\n",
		},
		{
			Name: "getConnectedUsers: No user is connected",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetConnectedUsers", 0, 100).Return([]*storemodels.ConnectedUser{}, nil).Times(1)
			},
			ExpectedStatusCode: http.StatusOK,
		},
		{
			Name: "getConnectedUsers: Users are connected",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetConnectedUsers", 0, 100).Return([]*storemodels.ConnectedUser{
					{
						MattermostUserID: testutils.GetUserID(),
						TeamsUserID:      testutils.GetTeamsUserID(),
						Email:            testutils.GetTestEmail(),
					},
				}, nil).Times(1)
			},
			ExpectedStatusCode: http.StatusOK,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			if test.ExpectedResult != "" {
				plugin.metricsService.(*metricsmocks.Metrics).On("IncrementHTTPErrors").Times(1)
			}

			mockAPI := &plugintest.API{}

			plugin.SetAPI(mockAPI)

			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(mockAPI)
			test.SetupStore(plugin.store.(*storemocks.Store))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/connected-users/download", nil)
			r.Header.Add(HeaderMattermostUserID, testutils.GetUserID())
			plugin.ServeHTTP(nil, w, r)

			result := w.Result()
			defer result.Body.Close()

			assert.NotNil(t, result)
			assert.Equal(test.ExpectedStatusCode, result.StatusCode)

			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(err)
			if test.ExpectedResult != "" {
				assert.Equal(test.ExpectedResult, string(bodyBytes))
			}
		})
	}
}

func TestDisconnect(t *testing.T) {
	for _, test := range []struct {
		Name               string
		SetupPlugin        func(*plugintest.API)
		SetupStore         func(*storemocks.Store)
		SetupMetrics       func(mockmetrics *metricsmocks.Metrics)
		ExpectedResult     string
		ExpectedStatusCode int
	}{
		{
			Name: "Disconnect: user successfully disconnected",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetUserID()).Return(testutils.GetID(), nil).Times(1)
				store.On("SetUserInfo", testutils.GetUserID(), testutils.GetID(), (*oauth2.Token)(nil)).Return(nil).Times(1)
			},
			SetupMetrics:       func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedResult:     "Your account has been disconnected.",
			ExpectedStatusCode: http.StatusOK,
		},
		{
			Name: "Disconnect: could not find the Teams user ID",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(1)
				api.On("LogError", "Unable to get Teams user ID from Mattermost user ID.", "UserID", testutils.GetUserID(), "Error", "could not find the Teams user ID").Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetUserID()).Return("", errors.New("could not find the Teams user ID")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			ExpectedResult:     "Unable to get Teams user ID from Mattermost user ID.\n",
			ExpectedStatusCode: http.StatusInternalServerError,
		},
		{
			Name: "Disconnect: could not get the token for MM user",
			SetupPlugin: func(api *plugintest.API) {
				api.On("LogError", "Unable to get the oauth token for the user.", "UserID", testutils.GetUserID(), "Error", "could not get the token for MM user").Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(nil, errors.New("could not get the token for MM user")).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetUserID()).Return(testutils.GetID(), nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			ExpectedResult:     "The account is not connected.\n",
			ExpectedStatusCode: http.StatusBadRequest,
		},
		{
			Name: "Disconnect: error occurred while setting the user info",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(1)
				api.On("LogError", "Error occurred while disconnecting the user.", "UserID", testutils.GetUserID(), "Error", "error occurred while setting the user info").Once()
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetUserID()).Return(testutils.GetID(), nil).Times(1)
				store.On("SetUserInfo", testutils.GetUserID(), testutils.GetID(), (*oauth2.Token)(nil)).Return(errors.New("error occurred while setting the user info")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			ExpectedResult:     "Error occurred while disconnecting the user.\n",
			ExpectedStatusCode: http.StatusInternalServerError,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			mockAPI := &plugintest.API{}

			plugin.SetAPI(mockAPI)
			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(mockAPI)
			test.SetupStore(plugin.store.(*storemocks.Store))
			test.SetupMetrics(plugin.metricsService.(*metricsmocks.Metrics))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/disconnect", nil)
			r.Header.Add(HeaderMattermostUserID, testutils.GetUserID())
			plugin.ServeHTTP(nil, w, r)

			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()

			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(err)

			bodyString := string(bodyBytes)
			assert.Equal(test.ExpectedResult, bodyString)
			assert.Equal(test.ExpectedStatusCode, result.StatusCode)
		})
	}
}

func TestGetLinkedChannels(t *testing.T) {
	for _, test := range []struct {
		Name               string
		SetupPlugin        func(*plugintest.API)
		SetupStore         func(*storemocks.Store)
		SetupClient        func(*clientmocks.Client)
		SetupMetrics       func(*metricsmocks.Metrics)
		ExpectedResult     string
		ExpectedStatusCode int
	}{
		{
			Name: "GetLinkedChannels: linked channels listed successfully",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionCreatePost).Return(true).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{}, nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("ListChannelLinksWithNames").Return([]*storemodels.ChannelLink{
					testutils.GetChannelLink(),
				}, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {
				c.On("GetChannelsInTeam", testutils.GetTeamsTeamID(), fmt.Sprintf("id in ('%s')", testutils.GetTeamsChannelID())).Return([]*clientmodels.Channel{
					{
						ID:          testutils.GetTeamsChannelID(),
						DisplayName: "mock-name",
					},
				}, nil).Times(1)
				c.On("GetTeams", fmt.Sprintf("id in ('%s')", testutils.GetTeamsTeamID())).Return([]*clientmodels.Team{
					{
						ID:          testutils.GetTeamsTeamID(),
						DisplayName: "mock-name",
					},
				}, nil).Times(1)
			},
			SetupMetrics:       func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedResult:     `[{"mattermostTeamID":"pqoeurndhroajdemq4nfmw","mattermostChannelID":"bnqnzipmnir4zkkj95ggba5pde","msTeamsTeamID":"test-teams-team-qplsnwere9nurernidte","msTeamsChannelID":"test-teams-channel","msTeamsTeamName":"mock-name","msTeamsChannelName":"mock-name"}]`,
			ExpectedStatusCode: http.StatusOK,
		},
		{
			Name: "GetLinkedChannels: error occurred while getting the linked channels",
			SetupPlugin: func(api *plugintest.API) {
				api.On("LogError", "Error occurred while getting the linked channels", "Error", "error occurred while getting the linked channels").Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("ListChannelLinksWithNames").Return(nil, errors.New("error occurred while getting the linked channels")).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			ExpectedResult:     "Error occurred while getting the linked channels\n",
			ExpectedStatusCode: http.StatusInternalServerError,
		},
		{
			Name: "GetLinkedChannels: error occurred while getting the MS Teams teams details",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionCreatePost).Return(true).Times(1)
				api.On("LogDebug", "Unable to get the MS Teams teams details", "Error", "error occurred while getting the MS Teams teams details")
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("ListChannelLinksWithNames").Return([]*storemodels.ChannelLink{
					testutils.GetChannelLink(),
				}, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {
				c.On("GetChannelsInTeam", testutils.GetTeamsTeamID(), fmt.Sprintf("id in ('%s')", testutils.GetTeamsChannelID())).Return([]*clientmodels.Channel{}, nil).Times(1)
				c.On("GetTeams", fmt.Sprintf("id in ('%s')", testutils.GetTeamsTeamID())).Return(nil, errors.New("error occurred while getting the MS Teams teams details")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			ExpectedResult:     "Unable to get the MS Teams teams details\n",
			ExpectedStatusCode: http.StatusInternalServerError,
		},
		{
			Name: "GetLinkedChannels: error occurred while getting channel details",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionCreatePost).Return(true).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, &model.AppError{
					Message: "error occurred while getting channel details",
				}).Times(1)
				api.On("LogError", "Error occurred while getting the channel details", "ChannelID", testutils.GetChannelID(), "Error", "error occurred while getting channel details").Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("ListChannelLinksWithNames").Return([]*storemodels.ChannelLink{
					testutils.GetChannelLink(),
				}, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {
				c.On("GetChannelsInTeam", testutils.GetTeamsTeamID(), fmt.Sprintf("id in ('%s')", testutils.GetTeamsChannelID())).Return([]*clientmodels.Channel{
					{
						ID:          testutils.GetTeamsChannelID(),
						DisplayName: "mock-name",
					},
				}, nil).Times(1)
				c.On("GetTeams", fmt.Sprintf("id in ('%s')", testutils.GetTeamsTeamID())).Return([]*clientmodels.Team{
					{
						ID:          testutils.GetTeamsTeamID(),
						DisplayName: "mock-name",
					},
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			ExpectedResult:     "Error occurred while getting the channel details\n",
			ExpectedStatusCode: http.StatusInternalServerError,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			mockAPI := &plugintest.API{}

			plugin.SetAPI(mockAPI)
			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(mockAPI)
			test.SetupStore(plugin.store.(*storemocks.Store))
			test.SetupClient(plugin.msteamsAppClient.(*clientmocks.Client))
			test.SetupMetrics(plugin.metricsService.(*metricsmocks.Metrics))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/linked-channels", nil)
			r.Header.Add(HeaderMattermostUserID, testutils.GetUserID())
			queryParams := url.Values{
				QueryParamPerPage: {fmt.Sprint(DefaultPerPage)},
				QueryParamPage:    {fmt.Sprint(DefaultPage)},
			}

			r.URL.RawQuery = queryParams.Encode()
			plugin.ServeHTTP(nil, w, r)
			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()

			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(err)

			bodyString := string(bodyBytes)
			assert.Equal(test.ExpectedResult, bodyString)
			assert.Equal(test.ExpectedStatusCode, result.StatusCode)
		})
	}
}

func TestGetMSTeamsTeamList(t *testing.T) {
	for _, test := range []struct {
		Name               string
		SetupPlugin        func(*plugintest.API)
		SetupStore         func(*storemocks.Store)
		SetupClient        func(*clientmocks.Client)
		SetupMetrics       func(*metricsmocks.Metrics)
		ExpectedResult     string
		ExpectedStatusCode int
	}{
		{
			Name:        "GetMSTeamsTeamList: MS Teams team listed successfully",
			SetupPlugin: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {
				c.On("ListTeams").Return([]*clientmodels.Team{
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
			SetupMetrics:       func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedResult:     `[{"ID":"mockTeamsTeamID-1","DisplayName":"mockDisplayName-1","Description":"mockDescription-1"},{"ID":"mockTeamsTeamID-2","DisplayName":"mockDisplayName-2","Description":"mockDescription-2"}]`,
			ExpectedStatusCode: http.StatusOK,
		},
		{
			Name: "GetMSTeamsTeamList: error occurred while getting MS Teams teams",
			SetupPlugin: func(api *plugintest.API) {
				api.On("LogError", "Unable to get the MS Teams teams", "Error", "error occurred while getting MS Teams teams").Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {
				c.On("ListTeams").Return(nil, errors.New("error occurred while getting MS Teams teams")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			ExpectedResult:     "Error occurred while fetching the MS Teams teams.\n",
			ExpectedStatusCode: http.StatusInternalServerError,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			mockAPI := &plugintest.API{}

			mockAPI.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(1)

			plugin.SetAPI(mockAPI)
			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(plugin.API.(*plugintest.API))
			test.SetupStore(plugin.store.(*storemocks.Store))
			test.SetupClient(plugin.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(plugin.metricsService.(*metricsmocks.Metrics))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/msteams/teams", nil)
			r.Header.Add(HeaderMattermostUserID, testutils.GetUserID())
			queryParams := url.Values{
				QueryParamPerPage: {fmt.Sprint(DefaultPerPage)},
				QueryParamPage:    {fmt.Sprint(DefaultPage)},
			}

			r.URL.RawQuery = queryParams.Encode()
			plugin.ServeHTTP(nil, w, r)
			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()

			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(err)

			bodyString := string(bodyBytes)
			assert.Equal(test.ExpectedResult, bodyString)
			assert.Equal(result.StatusCode, test.ExpectedStatusCode)
		})
	}
}

func TestGetMSTeamsTeamChannels(t *testing.T) {
	for _, test := range []struct {
		Name               string
		SetupPlugin        func(*plugintest.API)
		SetupStore         func(*storemocks.Store)
		SetupClient        func(*clientmocks.Client)
		SetupMetrics       func(*metricsmocks.Metrics)
		QueryParamTeamID   string
		ExpectedResult     string
		ExpectedStatusCode int
	}{
		{
			Name: "GetMSTeamsTeamChannels: MS Teams team channels listed successfully",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {
				c.On("ListChannels", testutils.GetTeamsTeamID()).Return([]*clientmodels.Channel{
					{
						ID:          "mockTeamsChannelID-1",
						DisplayName: "mockDisplayName-1",
						Description: "mockDescription-1",
					},
					{
						ID:          "mockTeamsChannelID-2",
						DisplayName: "mockDisplayName-2",
						Description: "mockDescription-2",
					},
				}, nil).Times(1)
			},
			SetupMetrics:       func(mockmetrics *metricsmocks.Metrics) {},
			QueryParamTeamID:   testutils.GetTeamsTeamID(),
			ExpectedResult:     `[{"ID":"mockTeamsChannelID-1","DisplayName":"mockDisplayName-1","Description":"mockDescription-1"},{"ID":"mockTeamsChannelID-2","DisplayName":"mockDisplayName-2","Description":"mockDescription-2"}]`,
			ExpectedStatusCode: http.StatusOK,
		},
		{
			Name: "GetMSTeamsTeamChannels: error occurred while getting MS Teams team channels",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(1)
				api.On("LogError", "Unable to get the channels for MS Teams team", "TeamID", testutils.GetTeamsTeamID(), "Error", "error occurred while getting MS Teams team channels").Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {
				c.On("ListChannels", testutils.GetTeamsTeamID()).Return(nil, errors.New("error occurred while getting MS Teams team channels")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			QueryParamTeamID:   testutils.GetTeamsTeamID(),
			ExpectedResult:     "Error occurred while fetching the MS Teams team channels.\n",
			ExpectedStatusCode: http.StatusInternalServerError,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			mockAPI := &plugintest.API{}

			plugin.SetAPI(mockAPI)
			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(mockAPI)
			test.SetupStore(plugin.store.(*storemocks.Store))
			test.SetupClient(plugin.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(plugin.metricsService.(*metricsmocks.Metrics))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/msteams/teams/%s/channels", testutils.GetTeamsTeamID()), nil)
			r.Header.Add(HeaderMattermostUserID, testutils.GetUserID())
			queryParams := url.Values{
				QueryParamPerPage: {fmt.Sprint(DefaultPerPage)},
				QueryParamPage:    {fmt.Sprint(DefaultPage)},
			}

			r.URL.RawQuery = queryParams.Encode()
			plugin.ServeHTTP(nil, w, r)
			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()

			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(err)

			bodyString := string(bodyBytes)
			assert.Equal(test.ExpectedResult, bodyString)
			assert.Equal(result.StatusCode, test.ExpectedStatusCode)
		})
	}
}

func TestLinkChannels(t *testing.T) {
	for _, test := range []struct {
		Name               string
		SetupPlugin        func(*plugintest.API)
		SetupStore         func(*storemocks.Store)
		SetupClient        func(*clientmocks.Client)
		SetupMetrics       func(*metricsmocks.Metrics)
		ExpectedResult     string
		ExpectedStatusCode int
		Body               string
	}{
		{
			Name: "LinkChannels: channels linked successfully",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(&model.Channel{
					Type: model.ChannelTypeOpen,
				}, nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(true).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("CheckEnabledTeamByTeamID", testutils.GetTeamID()).Return(true).Times(1)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetLinkByMSTeamsChannelID", testutils.GetTeamsTeamID(), testutils.GetTeamsChannelID()).Return(nil, nil).Times(1)
				store.On("StoreChannelLink", mock.Anything).Return(nil).Times(1)
				store.On("BeginTx").Return(&sql.Tx{}, nil).Times(1)
				store.On("SaveChannelSubscription", &sql.Tx{}, mock.AnythingOfType("storemodels.ChannelSubscription")).Return(nil).Times(1)
				store.On("CommitTx", &sql.Tx{}).Return(nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {
				c.On("GetChannelInTeam", testutils.GetTeamsTeamID(), testutils.GetTeamsChannelID()).Return(&clientmodels.Channel{}, nil)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveSubscription", metrics.SubscriptionConnected).Times(1)
			},
			ExpectedResult:     "Channels linked successfully",
			ExpectedStatusCode: http.StatusCreated,
			Body:               testutils.GetLinkChannelsPayload(testutils.GetTeamID(), testutils.GetChannelID(), testutils.GetTeamsTeamID(), testutils.GetTeamsChannelID()),
		},
		{
			Name: "LinkChannels: error in unmarshaling payload",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(1)
				api.On("LogError", "Error occurred while unmarshaling link channels payload.", "Error", "invalid character 'r' looking for beginning of object key string").Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			ExpectedResult:     "Error occurred while unmarshaling link channels payload.\n",
			ExpectedStatusCode: http.StatusBadRequest,
			Body: `{
				random
			}`,
		},
		{
			Name: "LinkChannels: invalid payload",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(1)
				api.On("LogError", "Invalid channel link payload.", "Error", "mattermost team ID is required").Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			ExpectedResult:     "Invalid channel link payload.\n",
			ExpectedStatusCode: http.StatusBadRequest,
			Body:               testutils.GetLinkChannelsPayload("", testutils.GetChannelID(), testutils.GetTeamsTeamID(), testutils.GetTeamsChannelID()),
		},
		{
			Name: "LinkChannels: error occurred while linking channels",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, &model.AppError{Message: "error occurred while getting channel details"}).Times(1)
				api.On("LogError", "Unable to get the current channel details.", "ChannelID", testutils.GetChannelID(), "Error", "error occurred while getting channel details").Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			ExpectedResult:     "Unable to get the current channel details.\n",
			ExpectedStatusCode: http.StatusInternalServerError,
			Body:               testutils.GetLinkChannelsPayload(testutils.GetTeamID(), testutils.GetChannelID(), testutils.GetTeamsTeamID(), testutils.GetTeamsChannelID()),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			mockAPI := &plugintest.API{}

			plugin.SetAPI(mockAPI)
			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(mockAPI)
			test.SetupStore(plugin.store.(*storemocks.Store))
			test.SetupClient(plugin.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(plugin.metricsService.(*metricsmocks.Metrics))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/channels/link", bytes.NewBufferString(test.Body))
			r.Header.Add(HeaderMattermostUserID, testutils.GetUserID())

			plugin.ServeHTTP(nil, w, r)
			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()

			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(err)

			bodyString := string(bodyBytes)
			assert.Equal(test.ExpectedResult, bodyString)
			assert.Equal(test.ExpectedStatusCode, result.StatusCode)
		})
	}
}

func TestUnlinkChannels(t *testing.T) {
	for _, test := range []struct {
		Name               string
		SetupPlugin        func(*plugintest.API)
		SetupStore         func(*storemocks.Store)
		SetupClient        func(*clientmocks.Client)
		SetupMetrics       func(*metricsmocks.Metrics)
		ExpectedResult     string
		ExpectedStatusCode int
	}{
		{
			Name: "UnlinkChannels: channels unlinked successfully",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeOpen), nil).Times(1)
				api.On("HasPermissionToChannel", testutils.GetUserID(), testutils.GetChannelID(), model.PermissionManageChannelRoles).Return(true).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(testutils.GetChannelLink(), nil).Once()
				store.On("DeleteLinkByChannelID", testutils.GetChannelID()).Return(nil).Times(1)
				store.On("GetChannelSubscriptionByTeamsChannelID", testutils.GetTeamsChannelID()).Return(&storemodels.ChannelSubscription{
					SubscriptionID: testutils.GetID(),
				}, nil).Times(1)
				store.On("DeleteSubscription", testutils.GetID()).Return(nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {
				c.On("DeleteSubscription", testutils.GetID()).Return(nil).Times(1)
			},
			SetupMetrics:       func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedResult:     "Channel unlinked successfully",
			ExpectedStatusCode: http.StatusOK,
		},
		{
			Name: "UnlinkChannels: error occurred while unlinking channels",
			SetupPlugin: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, &model.AppError{Message: "error occurred while unlinking channels"}).Times(1)
				api.On("LogError", "Unable to get the current channel details.", "ChannelID", testutils.GetChannelID(), "error", "error occurred while unlinking channels").Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&oauth2.Token{}, nil).Times(1)
			},
			SetupClient: func(c *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			ExpectedResult:     "Unable to get the current channel details.\n",
			ExpectedStatusCode: http.StatusInternalServerError,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			mockAPI := &plugintest.API{}

			plugin.SetAPI(mockAPI)
			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(mockAPI)
			test.SetupStore(plugin.store.(*storemocks.Store))
			test.SetupClient(plugin.msteamsAppClient.(*clientmocks.Client))
			test.SetupMetrics(plugin.metricsService.(*metricsmocks.Metrics))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/channels/%s/unlink", testutils.GetChannelID()), nil)
			r.Header.Add(HeaderMattermostUserID, testutils.GetUserID())

			plugin.ServeHTTP(nil, w, r)
			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()

			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(err)

			bodyString := string(bodyBytes)
			assert.Equal(test.ExpectedResult, bodyString)
			assert.Equal(test.ExpectedStatusCode, result.StatusCode)
		})
	}
}

func TestWhitelistUser(t *testing.T) {
	for _, test := range []struct {
		Name               string
		SetupPlugin        func(*plugintest.API)
		SetupStore         func(*storemocks.Store)
		SetupMetrics       func(*metricsmocks.Metrics)
		ExpectedResult     string
		ExpectedStatusCode int
	}{
		{
			Name: "WhitelistUser: unable to check user in the whitelist",
			SetupPlugin: func(api *plugintest.API) {
				api.On("LogError", "Error in checking if a user is present in whitelist", "UserID", testutils.GetUserID(), "Error", "unable to check user in the whitelist").Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("IsUserPresentInWhitelist", testutils.GetUserID()).Return(false, errors.New("unable to check user in the whitelist")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("IncrementHTTPErrors").Times(1)
			},
			ExpectedResult:     "error in checking if a user is present in whitelist\n",
			ExpectedStatusCode: http.StatusInternalServerError,
		},
		{
			Name:        "WhitelistUser: user is not present in whitelist",
			SetupPlugin: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("IsUserPresentInWhitelist", testutils.GetUserID()).Return(false, nil).Times(1)
			},
			SetupMetrics:       func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedResult:     `{"presentInWhitelist":false}`,
			ExpectedStatusCode: http.StatusOK,
		},
		{
			Name:        "WhitelistUser: user present in whitelist",
			SetupPlugin: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("IsUserPresentInWhitelist", testutils.GetUserID()).Return(true, nil).Times(1)
			},
			SetupMetrics:       func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedResult:     `{"presentInWhitelist":true}`,
			ExpectedStatusCode: http.StatusOK,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			plugin := newTestPlugin(t)
			mockAPI := &plugintest.API{}

			plugin.SetAPI(mockAPI)
			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(mockAPI)
			test.SetupStore(plugin.store.(*storemocks.Store))
			test.SetupMetrics(plugin.metricsService.(*metricsmocks.Metrics))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/whitelist-user", nil)
			r.Header.Add(HeaderMattermostUserID, testutils.GetUserID())
			plugin.ServeHTTP(nil, w, r)

			result := w.Result()
			assert.NotNil(t, result)
			defer result.Body.Close()

			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(err)

			bodyString := string(bodyBytes)
			assert.Equal(test.ExpectedResult, bodyString)
			assert.Equal(test.ExpectedStatusCode, result.StatusCode)
		})
	}
}
