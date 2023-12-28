package ce2e

import (
	"context"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils/containere2e"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestMessageHasBeenPostedNewMessageE2E(t *testing.T) {
	t.Parallel()
	mattermost, store, tearDown := containere2e.NewE2ETestPlugin(t)
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

	err = store.SetUserInfo(user.Id, "ms-user-id", &oauth2.Token{})
	require.NoError(t, err)

	t.Run("Without Channel Link", func(t *testing.T) {
		var newPost *model.Post
		newPost, _, err = client.CreatePost(context.Background(), &post)
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		_, err = store.GetPostInfoByMattermostID(newPost.Id)
		require.Error(t, err)
	})

	t.Run("Everything OK", func(t *testing.T) {
		containere2e.ResetMSTeamsClientMock(t, client)
		containere2e.MockMSTeamsClient(t, client, "GetChannelInTeam", "Channel", clientmodels.Channel{ID: "ms-channel-id"}, "")
		containere2e.MockMSTeamsClient(t, client, "SendMessageWithAttachments", "Message", clientmodels.Message{ID: "ms-post-id", LastUpdateAt: time.Now()}, "")

		_, _, err = client.ExecuteCommand(context.Background(), channel.Id, "/msteams-sync link ms-team-id ms-channel-id")
		require.NoError(t, err)

		var newPost *model.Post
		newPost, _, err = client.CreatePost(context.Background(), &post)
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		var postInfo *storemodels.PostInfo
		postInfo, err = store.GetPostInfoByMattermostID(newPost.Id)
		require.NoError(t, err)
		require.Equal(t, postInfo.MSTeamsID, "ms-post-id")
	})

	t.Run("Failing to deliver message to MSTeams", func(t *testing.T) {
		containere2e.ResetMSTeamsClientMock(t, client)
		containere2e.MockMSTeamsClient(t, client, "GetChannelInTeam", "Channel", clientmodels.Channel{ID: "ms-channel-id"}, "")
		containere2e.MockMSTeamsClient(t, client, "SendMessageWithAttachments", "Message", nil, "Unable to send the message")

		_, _, err = client.ExecuteCommand(context.Background(), channel.Id, "/msteams-sync link ms-team-id ms-channel-id")
		require.NoError(t, err)

		newPost, _, err := client.CreatePost(context.Background(), &post)
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		logs, err := mattermost.GetLogs(context.Background(), 10)
		require.NoError(t, err)

		require.Contains(t, logs, "Error creating post on MS Teams")
		require.Contains(t, logs, "Unable to handle message sent")

		_, err = store.GetPostInfoByMattermostID(newPost.Id)
		require.Error(t, err)
	})
}
