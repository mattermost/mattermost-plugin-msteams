//go:build clientMock
// +build clientMock

package main

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils/testmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"
)

var clientMock *mocks.Client

func (a *API) registerClientMock() {
	a.router.HandleFunc("/add-mock/{method:.*}", a.addMSTeamsClientMock).Methods(http.MethodPost)
	a.router.HandleFunc("/reset-mocks", a.resetMSTeamsClientMocks).Methods(http.MethodPost)
}

// resetMSTeamsClientMocks resets the msteams client mocks (for testing only)
func (a *API) resetMSTeamsClientMocks(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		a.p.API.LogWarn("Not authorized")
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	if !a.p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
		a.p.API.LogWarn("Insufficient permissions", "user_id", userID)
		http.Error(w, "not able to authorize the user", http.StatusForbidden)
		return
	}

	clientMock = nil
	newMock := getClientMock(a.p)
	a.p.msteamsAppClient = newMock
	w.WriteHeader(http.StatusOK)
}

// addMSTeamsClientMock adds a new msteams client function mock (for testing only)
func (a *API) addMSTeamsClientMock(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		a.p.API.LogWarn("Not authorized")
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	if !a.p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
		a.p.API.LogWarn("Insufficient permissions", "user_id", userID)
		http.Error(w, "not able to authorize the user", http.StatusForbidden)
		return
	}

	params := mux.Vars(r)
	methodName := params["method"]

	mockClient := a.p.clientBuilderWithToken("", "", "", "", nil, nil).(*mocks.Client)
	method, found := reflect.TypeOf(mockClient).MethodByName(methodName)
	if !found {
		a.p.API.LogError("Unable to mock the method, method not found", "MethodName", methodName)
		http.Error(w, "method not found", http.StatusNotFound)
		return
	}

	paramsCount := method.Type.NumIn()
	parameters := []interface{}{}
	for x := 0; x < paramsCount; x++ {
		parameters = append(parameters, mock.Anything)
	}

	var mockCall testmodels.MockCallReturns
	err := json.NewDecoder(r.Body).Decode(&mockCall)
	if err != nil {
		http.Error(w, "unable to mock the method", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	var returnErr error
	if mockCall.Err != "" {
		returnErr = errors.New(mockCall.Err)
	}

	var output any

	data, err := json.Marshal(mockCall.Returns)
	if err != nil {
		http.Error(w, "unable to mock the method", http.StatusBadRequest)
		return
	}

	switch mockCall.ReturnType {
	case "":
		output = nil
	case "Chat":
		output = &clientmodels.Chat{}
		err = json.Unmarshal(data, &output)
	case "ChatMember":
		output = &clientmodels.ChatMember{}
		err = json.Unmarshal(data, &output)
	case "Attachment":
		output = &clientmodels.Attachment{}
		err = json.Unmarshal(data, &output)
	case "Reaction":
		output = &clientmodels.Reaction{}
		err = json.Unmarshal(data, &output)
	case "Mention":
		output = &clientmodels.Mention{}
		err = json.Unmarshal(data, &output)
	case "Message":
		output = &clientmodels.Message{}
		err = json.Unmarshal(data, &output)
	case "Subscription":
		output = &clientmodels.Subscription{}
		err = json.Unmarshal(data, &output)
	case "Channel":
		output = &clientmodels.Channel{}
		err = json.Unmarshal(data, &output)
	case "User":
		output = &clientmodels.User{}
		err = json.Unmarshal(data, &output)
	case "Team":
		output = &clientmodels.Team{}
		err = json.Unmarshal(data, &output)
	case "ActivityIds":
		output = &clientmodels.ActivityIds{}
		err = json.Unmarshal(data, &output)
	}
	if err != nil {
		http.Error(w, "unable to mock the method", http.StatusBadRequest)
		return
	}

	returns := method.Type.NumOut()
	switch returns {
	case 0:
		a.p.API.LogDebug("mocking", "method", methodName)
		mockClient.On(methodName, parameters...)
	case 1:
		a.p.API.LogDebug("mocking", "method", methodName, "output", output)
		mockClient.On(methodName, parameters...).Return(output)
	case 2:
		a.p.API.LogDebug("mocking", "method", methodName, "output", output, "returnErr", returnErr)
		mockClient.On(methodName, parameters...).Return(output, returnErr)
	case 3:
		output1 := int64(mockCall.Returns.([]interface{})[0].(int))
		output2 := mockCall.Returns.([]interface{})[0].(string)
		a.p.API.LogDebug("mocking", "method", methodName, "output1", output1, "output2", output2, "returnErr", returnErr)
		mockClient.On(methodName, parameters...).Return(output1, output2, returnErr)
	}

	w.WriteHeader(http.StatusOK)
}

func getClientMock(p *Plugin) *mocks.Client {
	p.API.LogInfo("Using mock client")

	if clientMock != nil {
		return clientMock
	}
	newMock := mocks.Client{}
	newMock.On("ClearSubscriptions").Return(nil)
	newMock.On("RefreshToken", mock.Anything).Return(&oauth2.Token{}, nil)
	newMock.On("RefreshSubscriptionsPeriodically", mock.Anything, mock.Anything).Return(nil)
	newMock.On("SubscribeToChannels", mock.Anything, p.getConfiguration().WebhookSecret, "").Return("channel-subscription-id", nil)
	newMock.On("SubscribeToChats", mock.Anything, p.getConfiguration().WebhookSecret, true, "").Return(&clientmodels.Subscription{ID: "chats-subscription-id"}, nil)
	newMock.On("SubscribeToChannel", mock.Anything, mock.Anything, mock.Anything, p.getConfiguration().WebhookSecret, "").Return(&clientmodels.Subscription{ID: "channel-subscription-id"}, nil)
	newMock.On("ListSubscriptions").Return([]*clientmodels.Subscription{}, nil)
	newMock.On("GetAppCredentials", mock.Anything).Return([]clientmodels.Credential{}, nil)
	clientMock = &newMock
	return clientMock
}
