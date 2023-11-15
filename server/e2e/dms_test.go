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

func TestSendMessageAndReplyToMSTeamsDirectMessage(t *testing.T) {
	setup(t)
	generatedMessage := uuid.New().String()
	generatedReply := uuid.New().String()

	newMessage, err := msClient.SendChat(testCfg.MSTeams.ChatId, generatedMessage, nil, nil, nil)
	require.NoError(t, err)

	_, err = msClient.SendChat(testCfg.MSTeams.ChatId, generatedReply, newMessage, nil, nil)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)
	posts, _, err := mmClient.GetPostsForChannel(context.Background(), testCfg.Mattermost.DmId, 0, 10, "", false, false)
	require.NoError(t, err)

	var mattermostNewMessage *model.Post
	var mattermostNewReply *model.Post
	for _, post := range posts.Posts {
		if strings.Contains(post.Message, generatedMessage) {
			mattermostNewMessage = post
		}
		if strings.Contains(post.Message, generatedReply) {
			mattermostNewReply = post
		}
	}
	require.NotNil(t, mattermostNewMessage)
	require.NotNil(t, mattermostNewReply)
	require.Equal(t, mattermostNewReply.RootId, mattermostNewMessage.Id)
}

func TestSendMessageAndReplyToMattermostDirectMessage(t *testing.T) {
	setup(t)
	startTime := time.Now()
	generatedMessage := uuid.New().String()
	me, _, err := mmClient.GetMe(context.Background(), "")
	require.NoError(t, err)
	post := &model.Post{
		Message:   generatedMessage,
		ChannelId: testCfg.Mattermost.DmId,
		UserId:    me.Id,
	}
	newPost, _, err := mmClient.CreatePost(context.Background(), post)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	generatedReply := uuid.New().String()
	replyPost := &model.Post{
		Message:   generatedReply,
		ChannelId: testCfg.Mattermost.DmId,
		UserId:    me.Id,
		RootId:    newPost.Id,
	}
	_, _, err = mmClient.CreatePost(context.Background(), replyPost)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	msTeamsMessages, err := msClient.ListChatMessages(testCfg.MSTeams.ChatId, startTime)
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
	require.Contains(t, msteamsNewReply.Text, msteamsNewMessage.ID)
}
