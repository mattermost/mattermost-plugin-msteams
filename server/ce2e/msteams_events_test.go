package ce2e

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils/containere2e"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sendActivity(t *testing.T, client *model.Client4, activity msteams.Activity) error {
	data, err := json.Marshal(map[string][]msteams.Activity{"Value": {activity}})
	if err != nil {
		return err
	}

	resp, err := client.DoAPIRequest(context.Background(), http.MethodPost, client.URL+"/plugins/com.mattermost.msteams-sync/changes", string(data), "")
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return errors.New("unexpected status code")
	}
	return nil
}

func TestNewMSTeamsDirectMessage(t *testing.T) {
	mattermost, store, mockClient, tearDown := containere2e.NewE2ETestPlugin(t)
	defer tearDown()

	client, err := mattermost.GetAdminClient(context.Background())
	require.NoError(t, err)

	user, _, err := client.GetMe(context.Background(), "")
	require.NoError(t, err)

	err = mattermost.CreateUser(context.Background(), "otheruser@mattermost.com", "otheruser", "password")
	require.NoError(t, err)

	err = mattermost.AddUserToTeam(context.Background(), "otheruser", "test")
	require.NoError(t, err)

	otherUser, _, err := client.GetUserByUsername(context.Background(), "otheruser", "")
	require.NoError(t, err)

	dm, _, err := client.CreateDirectChannel(context.Background(), user.Id, otherUser.Id)
	require.NoError(t, err)

	err = store.SetUserInfo(user.Id, "ms-user-id", &fakeToken)
	require.NoError(t, err)

	err = store.SetUserInfo(otherUser.Id, "ms-otheruser-id", nil)
	require.NoError(t, err)

	err = store.SaveGlobalSubscription(storemodels.GlobalSubscription{
		SubscriptionID: "test-subscription-id",
		Type:           "allChats",
		ExpiresOn:      time.Now().Add(time.Hour),
		Secret:         "webhook-secret",
		Certificate:    "",
	})
	require.NoError(t, err)

	team, _, err := client.GetTeamByName(context.Background(), "test", "")
	require.NoError(t, err)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		suggestions, _, _ := client.ListCommandAutocompleteSuggestions(context.Background(), "/msteams", team.Id)
		assert.Len(c, suggestions, 1)
	}, 10*time.Second, 500*time.Millisecond)

	ttCases := []struct {
		name               string
		activity           msteams.Activity
		postText           string
		mock               func()
		expectPostCreation bool
	}{
		{
			name: "Valid new message without encrypted content",
			activity: msteams.Activity{
				Resource:    "chats('msteams-chat-id')/messages('msteams-message-id')",
				ClientState: "webhook-secret",
				ChangeType:  "created",
			},
			postText: "test-1",
			mock: func() {
				require.NoError(t, mockClient.Get("get-chat", "/v1.0/chats/msteams-chat-id", map[string]any{
					"@odata.context":      "https://graph.microsoft.com/v1.0/$metadata#chats/$entity",
					"id":                  "msteams-chat-id",
					"createdDateTime":     time.Now().Format(time.RFC3339),
					"lastUpdatedDateTime": time.Now().Format(time.RFC3339),
					"chatType":            "oneOnOne",
					"tenantId":            "tenant-id",
					"members": []map[string]any{
						{
							"@odata.type": "#microsoft.graph.aadUserConversationMember",
							"id":          "xxxxxxxxxxxx",
							"roles":       []string{"owner"},
							"displayName": user.Username,
							"userId":      "ms-user-id",
							"email":       user.Email,
							"tenantId":    "tenant-id",
						},
						{
							"@odata.type": "#microsoft.graph.aadUserConversationMember",
							"id":          "yyyyyyyyyyyy",
							"roles":       []string{"owner"},
							"displayName": otherUser.Username,
							"userId":      "ms-otheruser-id",
							"email":       otherUser.Email,
							"tenantId":    "tenant-id",
						},
					},
				}))
				require.NoError(t, mockClient.Get("get-user", "/v1.0/users/ms-user-id", map[string]any{
					"displayName": user.Username,
					"mail":        user.Email,
					"id":          "ms-user-id",
				}))
				require.NoError(t, mockClient.Get("get-other-user", "/v1.0/users/ms-otheruser-id", map[string]any{
					"displayName": otherUser.Username,
					"mail":        otherUser.Email,
					"id":          "ms-otheruser-id",
				}))
				require.NoError(t, mockClient.Get("get-message", "/v1.0/chats/msteams-chat-id/messages/msteams-message-id", map[string]any{
					"@odata.context":       "https://graph.microsoft.com/v1.0/$metadata#chats('19%3A8ea0e38b-efb3-4757-924a-5f94061cf8c2_97f62344-57dc-409c-88ad-c4af14158ff5%40unq.gbl.spaces')/messages/$entity",
					"id":                   "msteams-message-id",
					"etag":                 "msteams-message-id",
					"messageType":          "message",
					"createdDateTime":      "2021-02-02T18:19:52.105Z",
					"lastModifiedDateTime": "2021-02-02T18:19:52.105Z",
					"chatId":               "msteams-chat-id",
					"importance":           "normal",
					"locale":               "en-us",
					"from": map[string]any{
						"user": map[string]any{
							"@odata.type":      "#microsoft.graph.teamworkUserIdentity",
							"id":               "ms-user-id",
							"displayName":      user.Username,
							"userIdentityType": "aadUser",
							"tenantId":         "tenant-id",
						},
					},
					"body": map[string]any{
						"contentType": "text",
						"content":     "test-1",
					},
				}))
			},
			expectPostCreation: true,
		},
		{
			name: "Valid new message with encrypted content",
			activity: msteams.Activity{
				Resource:                       "chats('msteams-chat-id')/messages('msteams-message-id-2')",
				ClientState:                    "webhook-secret",
				ChangeType:                     "created",
				SubscriptionExpirationDateTime: time.Now().Add(time.Hour),
				SubscriptionID:                 "test-subscription-id",
				Content: func() []byte {
					result, _ := json.Marshal(map[string]any{
						"@odata.context":       "https://graph.microsoft.com/v1.0/$metadata#chats('19%3A8ea0e38b-efb3-4757-924a-5f94061cf8c2_97f62344-57dc-409c-88ad-c4af14158ff5%40unq.gbl.spaces')/messages/$entity",
						"id":                   "msteams-message-id-2",
						"etag":                 "msteams-message-id-2",
						"messageType":          "message",
						"createdDateTime":      "2021-02-02T18:19:52.105Z",
						"lastModifiedDateTime": "2021-02-02T18:19:52.105Z",
						"chatId":               "19:8ea0e38b-efb3-4757-924a-5f94061cf8c2_97f62344-57dc-409c-88ad-c4af14158ff5@unq.gbl.spaces",
						"importance":           "normal",
						"locale":               "en-us",
						"from": map[string]any{
							"user": map[string]any{
								"@odata.type":      "#microsoft.graph.teamworkUserIdentity",
								"id":               "ms-user-id",
								"displayName":      user.Username,
								"userIdentityType": "aadUser",
								"tenantId":         "tenant-id",
							},
						},
						"body": map[string]any{
							"contentType": "text",
							"content":     "test-2",
						},
					})
					return result
				}(),
			},
			postText: "test-2",
			mock: func() {
				require.NoError(t, mockClient.Get("get-chat", "/v1.0/chats/msteams-chat-id", map[string]any{
					"@odata.context":      "https://graph.microsoft.com/v1.0/$metadata#chats/$entity",
					"id":                  "msteams-chat-id",
					"createdDateTime":     time.Now().Format(time.RFC3339),
					"lastUpdatedDateTime": time.Now().Format(time.RFC3339),
					"chatType":            "oneOnOne",
					"tenantId":            "tenant-id",
					"members": []map[string]any{
						{
							"@odata.type": "#microsoft.graph.aadUserConversationMember",
							"id":          "xxxxxxxxxxxx",
							"roles":       []string{"owner"},
							"displayName": user.Username,
							"userId":      "ms-user-id",
							"email":       user.Email,
							"tenantId":    "tenant-id",
						},
						{
							"@odata.type": "#microsoft.graph.aadUserConversationMember",
							"id":          "yyyyyyyyyyyy",
							"roles":       []string{"owner"},
							"displayName": otherUser.Username,
							"userId":      "ms-otheruser-id",
							"email":       otherUser.Email,
							"tenantId":    "tenant-id",
						},
					},
				}))
				require.NoError(t, mockClient.Get("get-user", "/v1.0/users/ms-user-id", map[string]any{
					"displayName": user.Username,
					"mail":        user.Email,
					"id":          "ms-user-id",
				}))
				require.NoError(t, mockClient.Get("get-other-user", "/v1.0/users/ms-otheruser-id", map[string]any{
					"displayName": otherUser.Username,
					"mail":        otherUser.Email,
					"id":          "ms-otheruser-id",
				}))
			},
			expectPostCreation: true,
		},
	}

	for _, tc := range ttCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, mockClient.Reset())
			tc.mock()

			err := sendActivity(t, client, tc.activity)
			require.NoError(t, err)

			if tc.expectPostCreation {
				require.EventuallyWithT(t, func(c *assert.CollectT) {
					messages, _, err := client.GetPostsForChannel(context.Background(), dm.Id, 0, 1, "", false, false)
					assert.NoError(c, err)
					if assert.Len(c, messages.Order, 1) {
						assert.Equal(c, tc.postText, messages.Posts[messages.Order[0]].Message)
					}
				}, 2*time.Second, 20*time.Millisecond)
			} else {
				require.Never(t, func() bool {
					messages, _, err := client.GetPostsForChannel(context.Background(), dm.Id, 0, 1, "", false, false)
					if err != nil {
						return false
					}
					if len(messages.Order) != 1 {
						return false
					}
					if messages.Posts[messages.Order[0]].Message != tc.postText {
						return false
					}
					return true
				}, 2*time.Second, 20*time.Millisecond)
			}
		})
	}
}
