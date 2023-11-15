package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func TestStopPluginSendMessageAndReplyToMSTeamsLinkedChannelStartPlugin(t *testing.T) {
	setup(t)

	t.Log("Disabling the plugin")
	_, err := mmClientAdmin.DisablePlugin(context.Background(), "com.mattermost.msteams-sync")
	if err != nil {
		t.Fatal("Unable to disable the plugin", err)
	}
	defer mmClientAdmin.EnablePlugin(context.Background(), "com.mattermost.msteams-sync")

	generatedMessage := uuid.New().String()
	generatedReply := uuid.New().String()

	t.Log("Sending messages to MSTeams")
	newMessage, err := msClient.SendMessage(testCfg.MSTeams.ConnectedChannelTeamId, testCfg.MSTeams.ConnectedChannelId, "", generatedMessage)
	require.NoError(t, err)

	_, err = msClient.SendMessage(testCfg.MSTeams.ConnectedChannelTeamId, testCfg.MSTeams.ConnectedChannelId, newMessage.ID, generatedReply)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)

	t.Log("Verifying that the messages hasn't been synced to Mattermost")
	posts, _, err := mmClient.GetPostsForChannel(context.Background(), testCfg.Mattermost.ConnectedChannelId, 0, 10, "", false, false)
	require.NoError(t, err)

	var mattermostNewMessage *model.Post
	var mattermostNewReply *model.Post
	for _, post := range posts.Posts {
		if post.Message == generatedMessage {
			mattermostNewMessage = post
		}
		if post.Message == generatedReply {
			mattermostNewReply = post
		}
	}

	require.Nil(t, mattermostNewMessage)
	require.Nil(t, mattermostNewReply)

	t.Log("Enabling the plugin")
	_, err = mmClientAdmin.EnablePlugin(context.Background(), "com.mattermost.msteams-sync")
	if err != nil {
		t.Fatal("Unable to re-enable the plugin", err)
	}

	t.Log("Waiting for the plugin to and sync")
	time.Sleep(5 * time.Second)

	posts, _, err = mmClient.GetPostsForChannel(context.Background(), testCfg.Mattermost.ConnectedChannelId, 0, 10, "", false, false)
	require.NoError(t, err)

	for _, post := range posts.Posts {
		if post.Message == generatedMessage {
			mattermostNewMessage = post
		}
		if post.Message == generatedReply {
			mattermostNewReply = post
		}
	}

	require.NotNil(t, mattermostNewMessage)
	require.NotNil(t, mattermostNewReply)
	require.Equal(t, mattermostNewReply.RootId, mattermostNewMessage.Id)
}
