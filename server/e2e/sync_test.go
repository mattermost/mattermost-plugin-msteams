//go:build e2e
// +build e2e

package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
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
	newMessage, err := msClient.SendMessage(testCfg.MSTeams.ConnectedChannelTeamID, testCfg.MSTeams.ConnectedChannelID, "", generatedMessage)
	require.NoError(t, err)

	_, err = msClient.SendMessage(testCfg.MSTeams.ConnectedChannelTeamID, testCfg.MSTeams.ConnectedChannelID, newMessage.ID, generatedReply)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)

	t.Log("Verifying that the messages hasn't been synced to Mattermost")
	posts, _, err := mmClient.GetPostsForChannel(context.Background(), testCfg.Mattermost.ConnectedChannelID, 0, 10, "", false, false)
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

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		posts, _, err = mmClient.GetPostsForChannel(context.Background(), testCfg.Mattermost.ConnectedChannelID, 0, 10, "", false, false)
		require.NoError(t, err)

		for _, post := range posts.Posts {
			if post.Message == generatedMessage {
				mattermostNewMessage = post
			}
			if post.Message == generatedReply {
				mattermostNewReply = post
			}
		}

		assert.NotNil(c, mattermostNewMessage)
		assert.NotNil(c, mattermostNewReply)
		if mattermostNewMessage == nil || mattermostNewReply == nil {
			return
		}
		assert.Equal(c, mattermostNewReply.RootId, mattermostNewMessage.Id)
	}, 10*time.Second, 500*time.Millisecond)
}
