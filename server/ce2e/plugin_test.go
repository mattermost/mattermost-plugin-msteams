package ce2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils/containere2e"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

var fakeToken = oauth2.Token{Expiry: time.Now().Add(1 * time.Hour), AccessToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjozMDE2MjM5MDIyfQ.Kilb7fc4QwqfCad501vbAc861Ik1-30ytRtk8ZxEpgM"}

func TestMessageHasBeenPostedNewMessageE2E(t *testing.T) {
	t.Parallel()

	mattermost, store, mockClient, tearDown := containere2e.NewE2ETestPlugin(t)
	defer tearDown()

	client, err := mattermost.GetAdminClient(context.Background())
	require.NoError(t, err)

	user, _, err := client.GetMe(context.Background(), "")
	require.NoError(t, err)

	team, _, err := client.GetTeamByName(context.Background(), "test", "")
	require.NoError(t, err)

	channel, _, err := client.GetChannelByName(context.Background(), "town-square", team.Id, "")
	require.NoError(t, err)

	post := model.Post{
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
		UserId:    user.Id,
		ChannelId: channel.Id,
		Message:   "message",
	}

	err = store.SetUserInfo(user.Id, "ms-user-id", &fakeToken)
	require.NoError(t, err)

	t.Run("Without Channel Link", func(t *testing.T) {
		var newPost *model.Post
		newPost, _, err = client.CreatePost(context.Background(), &post)
		require.NoError(t, err)

		require.Never(t, func() bool {
			_, err = store.GetPostInfoByMattermostID(newPost.Id)
			return err == nil
		}, 1*time.Second, 50*time.Millisecond)
	})

	t.Run("Everything OK", func(t *testing.T) {
		if err = mockClient.Reset(); err != nil {
			t.Log(err)
		}

		err = mockClient.Get("/v1.0/teams/ms-team-id/channels/ms-channel-id", map[string]any{
			"id":              "ms-channel-id",
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "standard",
		})
		require.NoError(t, err)

		newPostID := model.NewId()
		err = mockClient.Post("/v1.0/teams/ms-team-id/channels/ms-channel-id/messages", map[string]any{
			"id":                   newPostID,
			"etag":                 "1616990032035",
			"messageType":          "message",
			"createdDateTime":      time.Now().Format(time.RFC3339),
			"lastModifiedDateTime": time.Now().Format(time.RFC3339),
			"importance":           "normal",
			"locale":               "en-us",
			"webUrl":               "https://teams.microsoft.com/l/message/ms-channel-id/test-message-id",
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
				"content":     "Hello World",
			},
			"channelIdentity": map[string]any{
				"teamId":    "ms-team-id",
				"channelId": "ms-channel-id",
			},
		})
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			suggestions, _, _ := client.ListCommandAutocompleteSuggestions(context.Background(), "/msteams", team.Id)
			return len(suggestions) > 0
		}, 5*time.Second, 500*time.Millisecond)

		_, _, err = client.ExecuteCommand(context.Background(), channel.Id, "/msteams link ms-team-id ms-channel-id")

		var newPost *model.Post
		newPost, _, err = client.CreatePost(context.Background(), &post)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			var postInfo *storemodels.PostInfo
			postInfo, err = store.GetPostInfoByMattermostID(newPost.Id)
			if err != nil {
				return false
			}
			if postInfo.MSTeamsID == newPostID {
				return true
			}
			return false
		}, 1*time.Second, 50*time.Millisecond)
	})

	t.Run("Failing to deliver message to MSTeams", func(t *testing.T) {
		if err = mockClient.Reset(); err != nil {
			t.Log(err)
		}

		err = mockClient.Get("/v1.0/teams/ms-team-id/channels/ms-channel-id", map[string]any{
			"id":              "ms-channel-id",
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "standard",
		})
		require.NoError(t, err)

		err = mockClient.MockError(http.MethodPost, "/v1.0/teams/ms-team-id/channels/ms-channel-id/messages")
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			suggestions, _, _ := client.ListCommandAutocompleteSuggestions(context.Background(), "/msteams", team.Id)
			return len(suggestions) > 0
		}, 5*time.Second, 500*time.Millisecond)

		_, _, err = client.ExecuteCommand(context.Background(), channel.Id, "/msteams link ms-team-id ms-channel-id")

		newPost, _, err := client.CreatePost(context.Background(), &post)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			var logs string
			logs, err = mattermost.GetLogs(context.Background(), 10)
			if err != nil {
				return false
			}
			if strings.Contains(logs, "Error creating post on MS Teams") && strings.Contains(logs, "Test bad request") {
				return true
			}
			return false
		}, 1*time.Second, 50*time.Millisecond)

		_, err = store.GetPostInfoByMattermostID(newPost.Id)
		require.Error(t, err)
	})
}

func TestMessageHasBeenPostedNewDirectMessageE2E(t *testing.T) {
	t.Parallel()

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

	post := model.Post{
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
		UserId:    user.Id,
		ChannelId: dm.Id,
		Message:   "message",
	}

	err = store.SetUserInfo(user.Id, "ms-user-id", &fakeToken)
	require.NoError(t, err)
	err = store.SetUserInfo(otherUser.Id, "ms-otheruser-id", nil)
	require.NoError(t, err)

	t.Run("Everything OK", func(t *testing.T) {
		if err = mockClient.Reset(); err != nil {
			t.Log(err)
		}

		err = mockClient.Post("/v1.0/chats", map[string]any{
			"id":              "ms-dm-id",
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "oneOnOne",
		})
		require.NoError(t, err)

		err = mockClient.Get("/v1.0/chats/ms-dm-id", map[string]any{
			"id":              "ms-dm-id",
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "oneOnOne",
		})
		require.NoError(t, err)

		newPostID := model.NewId()
		err = mockClient.Post("/v1.0/chats/ms-dm-id/messages", map[string]any{
			"id":                   newPostID,
			"etag":                 "1616990032035",
			"messageType":          "message",
			"createdDateTime":      time.Now().Format(time.RFC3339),
			"lastModifiedDateTime": time.Now().Format(time.RFC3339),
			"importance":           "normal",
			"locale":               "en-us",
			"webUrl":               "https://teams.microsoft.com/l/message/ms-dm-id/test-message-id",
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
				"content":     "Hello World",
			},
			"channelIdentity": map[string]any{
				"channelId": "ms-dm-id",
			},
		})
		require.NoError(t, err)

		var newPost *model.Post
		newPost, _, err = client.CreatePost(context.Background(), &post)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			var postInfo *storemodels.PostInfo
			postInfo, err = store.GetPostInfoByMattermostID(newPost.Id)
			if err != nil {
				return false
			}
			if postInfo.MSTeamsID == newPostID {
				return true
			}
			return false
		}, 1*time.Second, 50*time.Millisecond)
	})

	t.Run("Failing to deliver message to MSTeams", func(t *testing.T) {
		if err = mockClient.Reset(); err != nil {
			t.Log(err)
		}

		err = mockClient.Post("/v1.0/chats", map[string]any{
			"id":              "ms-dm-id",
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "oneOnOne",
		})
		require.NoError(t, err)

		err = mockClient.Get("/v1.0/chats/ms-dm-id", map[string]any{
			"id":              "ms-dm-id",
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "oneOnOne",
		})
		require.NoError(t, err)

		err = mockClient.MockError(http.MethodPost, "/v1.0/chats/ms-dm-id/messages")
		require.NoError(t, err)

		newPost, _, err := client.CreatePost(context.Background(), &post)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			var logs string
			logs, err = mattermost.GetLogs(context.Background(), 10)
			if err != nil {
				return false
			}
			if strings.Contains(logs, "Error creating post on MS Teams") && strings.Contains(logs, "Test bad request") {
				return true
			}
			return false
		}, 1*time.Second, 50*time.Millisecond)

		_, err = store.GetPostInfoByMattermostID(newPost.Id)
		require.Error(t, err)
	})
}
