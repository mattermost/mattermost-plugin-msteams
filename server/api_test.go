package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionValidation(t *testing.T) {
	plugin := newTestPlugin()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/?validationToken=test", nil)

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
	plugin := newTestPlugin()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))

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
	plugin := newTestPlugin()
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
						Resource:       "teams('team-id')/channels('channel-id')/messages('message-id')",
						SubscriptionId: "test",
					},
				},
			},
			func() {
				plugin.msteamsBotClient.(*mocks.Client).On("GetMessage", "team-id", "channel-id", "message-id").Return(&msteams.Message{}, nil).Times(1)
			},
			200,
			"",
		},
		{
			"Valid reply",
			Activities{
				Value: []msteams.Activity{
					{
						Resource:       "teams('team-id')/channels('channel-id')/messages('message-id')/replies('reply-id')",
						SubscriptionId: "test",
					},
				},
			},
			func() {
				plugin.msteamsBotClient.(*mocks.Client).On("GetReply", "team-id", "channel-id", "message-id", "reply-id").Return(&msteams.Message{}, nil).Times(1)
			},
			200,
			"",
		},
		{
			"Message not found",
			Activities{
				Value: []msteams.Activity{
					{
						Resource:       "teams('team-id')/channels('channel-id')/messages('message-id')",
						SubscriptionId: "test",
					},
				},
			},
			func() {
				plugin.msteamsBotClient.(*mocks.Client).On("GetMessage", "team-id", "channel-id", "message-id").Return(nil, errors.New("not found")).Times(1)
			},
			400,
			"not found\n\n",
		},
		{
			"Invalid activity",
			Activities{
				Value: []msteams.Activity{
					{
						Resource:       "test",
						SubscriptionId: "test",
					},
				},
			},
			func() {
				plugin.msteamsBotClient.(*mocks.Client).On("GetMessage", "", "", "").Return(nil, errors.New("test-error")).Times(1)
			},
			400,
			"test-error\n\n",
		},
	}
	for _, tc := range ttcases {
		t.Run(tc.Name, func(t *testing.T) {
			data, err := json.Marshal(tc.Activities)
			require.NoError(t, err)

			tc.PopulateMocks()

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(data))

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
	plugin := newTestPlugin()

	plugin.API.(*plugintest.API).On("KVGet", "avatar_user-id").Return([]byte("fake-avatar"), nil).Times(1)

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
	plugin := newTestPlugin()

	plugin.API.(*plugintest.API).On("KVGet", "avatar_user-id").Return(nil, &model.AppError{Message: "not-found"}).Times(1)
	plugin.msteamsAppClient.(*mocks.Client).On("GetUserAvatar", "user-id").Return([]byte("fake-avatar"), nil).Times(1)
	plugin.API.(*plugintest.API).On("KVSetWithExpiry", "avatar_user-id", []byte("fake-avatar"), int64(300)).Return(nil).Times(1)

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
	plugin := newTestPlugin()

	plugin.API.(*plugintest.API).On("KVGet", "avatar_user-id").Return(nil, &model.AppError{Message: "not-found"}).Times(1)
	plugin.msteamsAppClient.(*mocks.Client).On("GetUserAvatar", "user-id").Return(nil, errors.New("not-found")).Times(1)

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
