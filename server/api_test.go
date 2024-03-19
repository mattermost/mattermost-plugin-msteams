package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	metricsmocks "github.com/mattermost/mattermost-plugin-msteams/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

var fakeToken = oauth2.Token{Expiry: time.Now().Add(10 * time.Minute)}

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
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&fakeToken, nil).Times(1)
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
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&fakeToken, nil).Times(1)
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
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&fakeToken, nil).Times(1)
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
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&fakeToken, nil).Times(1)
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
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&fakeToken, nil)
			},
			http.StatusBadRequest,
			"Unable to get private key: invalid certificate key\n\n",
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
				plugin.store.(*storemocks.Store).On("GetTokenForMattermostUser", "bot-user-id").Return(&fakeToken, nil)
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
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
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
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("ListTeams").Return([]clientmodels.Team{
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
			},
			QueryParams: "mockData-1 mockData-2 mockData-3",
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
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
			},
			QueryParams: "mockData-1 mockData-2 mockData-3",
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("ListChannels", "mockData-3").Return([]clientmodels.Channel{
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
			ExpectedStatusCode: http.StatusSeeOther,
		},
		{
			Name: "connect: Error in storing the OAuth state",
			SetupPlugin: func(api *plugintest.API) {
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
			plugin.metricsService.(*metricsmocks.Metrics).On("IncrementHTTPErrors").Times(1)

			mockAPI := &plugintest.API{}
			testutils.MockLogs(mockAPI)
			plugin.SetAPI(mockAPI)

			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(mockAPI)
			test.SetupStore(plugin.store.(*storemocks.Store))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/connect", nil)
			r.Header.Add("Mattermost-User-Id", testutils.GetUserID())
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
			},
			SetupStore:         func(store *storemocks.Store) {},
			ExpectedStatusCode: http.StatusForbidden,
			ExpectedResult:     "not able to authorize the user\n",
		},
		{
			Name: "getConnectedUsers: Unable to get the list of connected users from the store",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
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
			testutils.MockLogs(mockAPI)

			plugin.SetAPI(mockAPI)
			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(mockAPI)
			test.SetupStore(plugin.store.(*storemocks.Store))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/connected-users", nil)
			r.Header.Add("Mattermost-User-Id", testutils.GetUserID())
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
			},
			SetupStore:         func(store *storemocks.Store) {},
			ExpectedStatusCode: http.StatusForbidden,
			ExpectedResult:     "not able to authorize the user\n",
		},
		{
			Name: "getConnectedUsers: Unable to get the list of connected users from the store",
			SetupPlugin: func(api *plugintest.API) {
				api.On("HasPermissionTo", testutils.GetUserID(), model.PermissionManageSystem).Return(true).Times(1)
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
			testutils.MockLogs(mockAPI)

			defer mockAPI.AssertExpectations(t)

			test.SetupPlugin(mockAPI)
			test.SetupStore(plugin.store.(*storemocks.Store))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/connected-users/download", nil)
			r.Header.Add("Mattermost-User-Id", testutils.GetUserID())
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
