package ce2e

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils/containere2e"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils/mmcontainer"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

const pluginID = "com.mattermost.msteams-sync"

var fakeToken = oauth2.Token{Expiry: time.Now().Add(1 * time.Hour), AccessToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjozMDE2MjM5MDIyfQ.Kilb7fc4QwqfCad501vbAc861Ik1-30ytRtk8ZxEpgM"}

func setUserDefaultPlatform(t *testing.T, mattermost *mmcontainer.MattermostContainer, user *model.User, platform string) {
	client, err := mattermost.GetClient(context.Background(), user.Username, "password")
	require.NoError(t, err)
	preferences, _, err := client.GetPreferences(context.Background(), user.Id)
	require.NoError(t, err)
	preferences = append(preferences, model.Preference{
		UserId:   user.Id,
		Category: "pp_" + pluginID,
		Name:     "platform",
		Value:    platform,
	})

	_, err = client.UpdatePreferences(context.Background(), user.Id, preferences)
	require.NoError(t, err)
}

func TestMessageHasBeenPostedNewMessageE2E(t *testing.T) {
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

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		suggestions, _, _ := client.ListCommandAutocompleteSuggestions(context.Background(), "/msteams", team.Id)
		assert.Len(c, suggestions, 1)
	}, 10*time.Second, 500*time.Millisecond)

	t.Run("Without Channel Link", func(t *testing.T) {
		var newPost *model.Post
		newPost, _, err = client.CreatePost(context.Background(), &post)
		assert.NoError(t, err)

		require.Never(t, func() bool {
			_, err = store.GetPostInfoByMattermostID(newPost.Id)
			return err == nil
		}, 1*time.Second, 50*time.Millisecond)
	})

	t.Run("Everything OK", func(t *testing.T) {
		require.NoError(t, mockClient.Reset())

		err = mockClient.Get("get-channel", "/v1.0/teams/ms-team-id/channels/ms-channel-id", map[string]any{
			"id":              "ms-channel-id",
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "standard",
		})
		require.NoError(t, err)

		newPostID := model.NewId()
		err = mockClient.Post("post-message", "/v1.0/teams/ms-team-id/channels/ms-channel-id/messages", map[string]any{
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

		_, _, err = client.ExecuteCommand(context.Background(), channel.Id, "/msteams link ms-team-id ms-channel-id")

		var newPost *model.Post
		newPost, _, err = client.CreatePost(context.Background(), &post)
		require.NoError(t, err)

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			var postInfo *storemodels.PostInfo
			postInfo, err = store.GetPostInfoByMattermostID(newPost.Id)
			if assert.NoError(c, err) {
				assert.Equal(c, newPostID, postInfo.MSTeamsID)
			}
		}, 1*time.Second, 50*time.Millisecond)
	})

	t.Run("Failing to deliver message to MSTeams", func(t *testing.T) {
		require.NoError(t, mockClient.Reset())

		err = mockClient.Get("get-channel", "/v1.0/teams/ms-team-id/channels/ms-channel-id", map[string]any{
			"id":              "ms-channel-id",
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "standard",
		})
		require.NoError(t, err)

		err = mockClient.MockError("failed-post-message", http.MethodPost, http.StatusBadRequest, "/v1.0/teams/ms-team-id/channels/ms-channel-id/messages")
		require.NoError(t, err)

		_, _, err = client.ExecuteCommand(context.Background(), channel.Id, "/msteams link ms-team-id ms-channel-id")

		newPost, _, err := client.CreatePost(context.Background(), &post)
		require.NoError(t, err)

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			var logs string
			logs, err = mattermost.GetLogs(context.Background(), 50)
			assert.NoError(c, err)
			assert.Contains(c, logs, "Error creating post on MS Teams")
			assert.Contains(c, logs, "Test bad request")
		}, 1*time.Second, 50*time.Millisecond)

		_, err = store.GetPostInfoByMattermostID(newPost.Id)
		require.Error(t, err)
	})
}

func TestMessageHasBeenPostedNewDirectMessageE2E(t *testing.T) {
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
		require.NoError(t, mockClient.Reset())

		err = mockClient.Post("create-chat", "/v1.0/chats", map[string]any{
			"id":              "ms-dm-id",
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "oneOnOne",
		})
		require.NoError(t, err)

		err = mockClient.Get("get-chat", "/v1.0/chats/ms-dm-id", map[string]any{
			"id":              "ms-dm-id",
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "oneOnOne",
		})
		require.NoError(t, err)

		newPostID := model.NewId()
		err = mockClient.Post("post-message", "/v1.0/chats/ms-dm-id/messages", map[string]any{
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

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			var postInfo *storemodels.PostInfo
			postInfo, err = store.GetPostInfoByMattermostID(newPost.Id)
			if assert.NoError(c, err) {
				assert.Equal(c, newPostID, postInfo.MSTeamsID)
			}
		}, 5*time.Second, 50*time.Millisecond)
	})

	t.Run("Failing to deliver message to MSTeams", func(t *testing.T) {
		require.NoError(t, mockClient.Reset())

		err = mockClient.Post("create-chat", "/v1.0/chats", map[string]any{
			"id":              "ms-dm-id",
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "oneOnOne",
		})
		require.NoError(t, err)

		err = mockClient.Get("get-chat", "/v1.0/chats/ms-dm-id", map[string]any{
			"id":              "ms-dm-id",
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "oneOnOne",
		})
		require.NoError(t, err)

		err = mockClient.MockError("failed-to-post-message", http.MethodPost, http.StatusBadRequest, "/v1.0/chats/ms-dm-id/messages")
		require.NoError(t, err)

		newPost, _, err := client.CreatePost(context.Background(), &post)
		require.NoError(t, err)

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			var logs string
			logs, err = mattermost.GetLogs(context.Background(), 10)
			assert.NoError(c, err)
			assert.Contains(c, logs, "Error creating post on MS Teams")
			assert.Contains(c, logs, "Test bad request")
		}, 1*time.Second, 50*time.Millisecond)

		_, err = store.GetPostInfoByMattermostID(newPost.Id)
		require.Error(t, err)
	})
}

func TestSelectiveSync(t *testing.T) {
	mattermost, store, mockClient, tearDown := containere2e.NewE2ETestPlugin(t)
	defer tearDown()

	adminClient, err := mattermost.GetAdminClient(context.Background())
	require.NoError(t, err)

	err = mattermost.CreateUser(context.Background(), "not-connected-user1@mattermost.com", "notconnected1", "password")
	require.NoError(t, err)

	err = mattermost.CreateUser(context.Background(), "mattermost-primary1@mattermost.com", "mmprimary1", "password")
	require.NoError(t, err)

	err = mattermost.CreateUser(context.Background(), "msteams-primary1@mattermost.com", "msteamsprimary1", "password")
	require.NoError(t, err)

	err = mattermost.CreateUser(context.Background(), "not-connected-user2@mattermost.com", "notconnected2", "password")
	require.NoError(t, err)

	err = mattermost.CreateUser(context.Background(), "mattermost-primary2@mattermost.com", "mmprimary2", "password")
	require.NoError(t, err)

	err = mattermost.CreateUser(context.Background(), "msteams-primary2@mattermost.com", "msteamsprimary2", "password")
	require.NoError(t, err)

	err = mattermost.CreateUser(context.Background(), "sysnthetic@mattermost.com", "msteams_synthetic", "password")
	require.NoError(t, err)

	err = mattermost.AddUserToTeam(context.Background(), "notconnected1", "test")
	require.NoError(t, err)
	err = mattermost.AddUserToTeam(context.Background(), "mmprimary1", "test")
	require.NoError(t, err)
	err = mattermost.AddUserToTeam(context.Background(), "msteamsprimary1", "test")
	require.NoError(t, err)
	err = mattermost.AddUserToTeam(context.Background(), "notconnected2", "test")
	require.NoError(t, err)
	err = mattermost.AddUserToTeam(context.Background(), "mmprimary2", "test")
	require.NoError(t, err)
	err = mattermost.AddUserToTeam(context.Background(), "msteamsprimary2", "test")
	require.NoError(t, err)
	err = mattermost.AddUserToTeam(context.Background(), "msteams_synthetic", "test")
	require.NoError(t, err)

	notConnected1, _, err := adminClient.GetUserByUsername(context.Background(), "notconnected1", "")
	require.NoError(t, err)
	notConnected2, _, err := adminClient.GetUserByUsername(context.Background(), "notconnected2", "")
	require.NoError(t, err)
	mmPrimary1, _, err := adminClient.GetUserByUsername(context.Background(), "mmprimary1", "")
	require.NoError(t, err)
	mmPrimary2, _, err := adminClient.GetUserByUsername(context.Background(), "mmprimary2", "")
	require.NoError(t, err)
	msteamsPrimary1, _, err := adminClient.GetUserByUsername(context.Background(), "msteamsprimary1", "")
	require.NoError(t, err)
	msteamsPrimary2, _, err := adminClient.GetUserByUsername(context.Background(), "msteamsprimary2", "")
	require.NoError(t, err)
	synthetic, _, err := adminClient.GetUserByUsername(context.Background(), "msteams_synthetic", "")
	require.NoError(t, err)

	err = store.SetUserInfo(mmPrimary1.Id, "ms-mmprimary1", &fakeToken)
	require.NoError(t, err)
	setUserDefaultPlatform(t, mattermost, mmPrimary1, "mattermost")
	err = store.SetUserInfo(mmPrimary2.Id, "ms-mmprimary2", &fakeToken)
	require.NoError(t, err)
	setUserDefaultPlatform(t, mattermost, mmPrimary2, "mattermost")
	err = store.SetUserInfo(msteamsPrimary1.Id, "ms-msteamsprimary1", &fakeToken)
	require.NoError(t, err)
	setUserDefaultPlatform(t, mattermost, msteamsPrimary1, "msteams")
	err = store.SetUserInfo(msteamsPrimary2.Id, "ms-msteamsprimary2", &fakeToken)
	require.NoError(t, err)
	setUserDefaultPlatform(t, mattermost, msteamsPrimary2, "msteams")
	err = store.SetUserInfo(synthetic.Id, "ms-msteams_synthetic", nil)
	require.NoError(t, err)

	conn, err := mattermost.PostgresConnection(context.Background())
	require.NoError(t, err)
	defer conn.Close()

	// Mark user as synthetic
	_, err = conn.Exec("UPDATE Users SET RemoteId = (SELECT remoteId FROM remoteclusters WHERE pluginid=$1) WHERE Username = 'msteams_synthetic'", pluginID)
	require.NoError(t, err)

	// team, _, err := adminClient.GetTeamByName(context.Background(), "test", "")
	// require.NoError(t, err)

	// require.EventuallyWithT(t, func(c *assert.CollectT) {
	// 	suggestions, _, _ := adminClient.ListCommandAutocompleteSuggestions(context.Background(), "/msteams", team.Id)
	// 	assert.Len(c, suggestions, 1)
	// }, 10*time.Second, 500*time.Millisecond)

	ttCases := []struct {
		name                         string
		fromUser                     *model.User
		toUser                       *model.User
		expectedWithSelectiveSync    bool
		expectedWithoutSelectiveSync bool
	}{
		{
			name:                         "from not connected to not connected",
			fromUser:                     notConnected1,
			toUser:                       notConnected2,
			expectedWithSelectiveSync:    false,
			expectedWithoutSelectiveSync: false,
		},
		{
			name:                         "from not connected to mmprimary",
			fromUser:                     notConnected1,
			toUser:                       mmPrimary1,
			expectedWithSelectiveSync:    false,
			expectedWithoutSelectiveSync: false,
		},
		{
			name:                         "from not connected to msteamsprimary",
			fromUser:                     notConnected1,
			toUser:                       msteamsPrimary1,
			expectedWithSelectiveSync:    false,
			expectedWithoutSelectiveSync: false,
		},
		{
			name:                         "from not connected to synthetic",
			fromUser:                     notConnected1,
			toUser:                       synthetic,
			expectedWithSelectiveSync:    false,
			expectedWithoutSelectiveSync: false,
		},
		{
			name:                         "from mmprimary to not connected",
			fromUser:                     mmPrimary1,
			toUser:                       notConnected2,
			expectedWithSelectiveSync:    false,
			expectedWithoutSelectiveSync: false,
		},
		{
			name:                         "from mmprimary to mmprimary",
			fromUser:                     mmPrimary1,
			toUser:                       mmPrimary2,
			expectedWithSelectiveSync:    false,
			expectedWithoutSelectiveSync: true,
		},
		{
			name:                         "from mmprimary to msteamsprimary",
			fromUser:                     mmPrimary1,
			toUser:                       msteamsPrimary1,
			expectedWithSelectiveSync:    true,
			expectedWithoutSelectiveSync: true,
		},
		{
			name:                         "from mmprimary to synthetic",
			fromUser:                     mmPrimary1,
			toUser:                       synthetic,
			expectedWithSelectiveSync:    true,
			expectedWithoutSelectiveSync: true,
		},
		{
			name:                         "from msteamsprimary to not connected",
			fromUser:                     msteamsPrimary1,
			toUser:                       notConnected2,
			expectedWithSelectiveSync:    false,
			expectedWithoutSelectiveSync: false,
		},
		{
			name:                         "from msteamsprimary to mmprimary",
			fromUser:                     msteamsPrimary1,
			toUser:                       mmPrimary1,
			expectedWithSelectiveSync:    true,
			expectedWithoutSelectiveSync: true,
		},
		{
			name:                         "from msteamsprimary to msteamsprimary",
			fromUser:                     msteamsPrimary1,
			toUser:                       msteamsPrimary2,
			expectedWithSelectiveSync:    false,
			expectedWithoutSelectiveSync: true,
		},
		{
			name:                         "from msteamsprimary to synthetic",
			fromUser:                     msteamsPrimary1,
			toUser:                       synthetic,
			expectedWithSelectiveSync:    false,
			expectedWithoutSelectiveSync: true,
		},
	}

	for _, enabledSelectiveSync := range []bool{false, true} {
		config, _, err := adminClient.GetConfig(context.Background())
		require.NoError(t, err)
		config.PluginSettings.Plugins[pluginID]["selectiveSync"] = enabledSelectiveSync
		_, _, err = adminClient.UpdateConfig(context.Background(), config)
		require.NoError(t, err)

		name := "selective sync disabled"
		if enabledSelectiveSync {
			name = "selective sync enabled"
		}

		t.Run(name, func(t *testing.T) {
			for _, tc := range ttCases {
				client, err := mattermost.GetClient(context.Background(), tc.fromUser.Username, "password")
				require.NoError(t, err)

				dm, _, err := client.CreateDirectChannel(context.Background(), tc.fromUser.Id, tc.toUser.Id)
				require.NoError(t, err)

				require.NoError(t, mockClient.Reset())

				newDMID := model.NewId()

				err = mockClient.Post("create-chat", "/v1.0/chats", map[string]any{
					"id":              newDMID,
					"createdDateTime": time.Now().Format(time.RFC3339),
					"displayName":     "test channel",
					"description":     "Test channel",
					"membershipType":  "oneOnOne",
				})
				require.NoError(t, err)

				err = mockClient.Get("get-chat", "/v1.0/chats/"+newDMID, map[string]any{
					"id":              newDMID,
					"createdDateTime": time.Now().Format(time.RFC3339),
					"displayName":     "test channel",
					"description":     "Test channel",
					"membershipType":  "oneOnOne",
				})
				require.NoError(t, err)

				newPostID := model.NewId()
				err = mockClient.Post("post-message", "/v1.0/chats/"+newDMID+"/messages", map[string]any{
					"id":                   newPostID,
					"messageType":          "message",
					"createdDateTime":      time.Now().Format(time.RFC3339),
					"lastModifiedDateTime": time.Now().Format(time.RFC3339),
					"from": map[string]any{
						"user": map[string]any{
							"@odata.type":      "#microsoft.graph.teamworkUserIdentity",
							"id":               "ms-" + tc.fromUser.Username,
							"displayName":      tc.fromUser.Username,
							"userIdentityType": "aadUser",
							"tenantId":         "tenant-id",
						},
					},
					"body": map[string]any{
						"contentType": "text",
						"content":     "Hello World",
					},
					"channelIdentity": map[string]any{
						"channelId": newDMID,
					},
				})
				require.NoError(t, err)

				post := model.Post{
					CreateAt:  model.GetMillis(),
					UpdateAt:  model.GetMillis(),
					UserId:    tc.fromUser.Id,
					ChannelId: dm.Id,
					Message:   "message",
				}

				t.Run(tc.name, func(t *testing.T) {
					_, _, err = client.CreatePost(context.Background(), &post)
					require.NoError(t, err)

					require.EventuallyWithT(t, func(c *assert.CollectT) {
						if enabledSelectiveSync && tc.expectedWithSelectiveSync || !enabledSelectiveSync && tc.expectedWithoutSelectiveSync {
							assert.NoError(c, mockClient.Assert("post-message", 1))
						} else {
							assert.NoError(c, mockClient.Assert("post-message", 0))
						}
					}, 5*time.Second, 50*time.Millisecond)
				})
			}
		})
	}
}
