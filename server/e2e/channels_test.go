package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func TestSendMessageAndReplyToMSTeamsLinkedChannel(t *testing.T) {
	setup(t)
	generatedMessage := uuid.New().String()
	generatedReply := uuid.New().String()
	newMessage, err := msClient.SendMessage(testCfg.MSTeams.ConnectedChannelTeamId, testCfg.MSTeams.ConnectedChannelId, "", generatedMessage)
	require.NoError(t, err)

	_, err = msClient.SendMessage(testCfg.MSTeams.ConnectedChannelTeamId, testCfg.MSTeams.ConnectedChannelId, newMessage.ID, generatedReply)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)
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
	require.NotNil(t, mattermostNewMessage)
	require.NotNil(t, mattermostNewReply)
	require.Equal(t, mattermostNewReply.RootId, mattermostNewMessage.Id)
}

func TestSendMessageAndReplyToMattermostLinkedChannel(t *testing.T) {
	setup(t)
	startTime := time.Now()
	generatedMessage := uuid.New().String()
	me, _, err := mmClient.GetMe(context.Background(), "")
	require.NoError(t, err)
	post := &model.Post{
		Message:   generatedMessage,
		ChannelId: testCfg.Mattermost.ConnectedChannelId,
		UserId:    me.Id,
	}
	newPost, _, err := mmClient.CreatePost(context.Background(), post)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	generatedReply := uuid.New().String()
	replyPost := &model.Post{
		Message:   generatedReply,
		ChannelId: testCfg.Mattermost.ConnectedChannelId,
		UserId:    me.Id,
		RootId:    newPost.Id,
	}
	_, _, err = mmClient.CreatePost(context.Background(), replyPost)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	msTeamsMessages, err := msClient.ListChannelMessages(testCfg.MSTeams.ConnectedChannelTeamId, testCfg.MSTeams.ConnectedChannelId, startTime)
	require.NoError(t, err)

	var msteamsNewMessage *clientmodels.Message
	var msteamsNewReply *clientmodels.Message
	for _, msg := range msTeamsMessages {
		if strings.Contains(msg.Text, generatedMessage) {
			msteamsNewMessage = msg
		}
		if strings.Contains(msg.Text, generatedReply) {
			msteamsNewReply = msg
		}
	}

	require.NotNil(t, msteamsNewMessage)
	require.NotNil(t, msteamsNewReply)
	require.Equal(t, msteamsNewReply.ReplyToID, msteamsNewMessage.ID)
}