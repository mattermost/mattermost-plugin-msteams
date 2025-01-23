// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

//go:build e2e
// +build e2e

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendMessageAndReplyToMSTeamsDirectMessage(t *testing.T) {
	setup(t)
	generatedMessage := uuid.New().String()
	generatedReply := uuid.New().String()

	newMessage, err := msClient.SendChat(testCfg.MSTeams.ChatID, generatedMessage, nil, nil, nil)
	require.NoError(t, err)

	_, err = msClient.SendChat(testCfg.MSTeams.ChatID, generatedReply, newMessage, nil, nil)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		posts, _, err := mmClient.GetPostsForChannel(context.Background(), testCfg.Mattermost.DmID, 0, 10, "", false, false)
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
		assert.NotNil(c, mattermostNewMessage)
		assert.NotNil(c, mattermostNewReply)
		if (mattermostNewMessage == nil) || (mattermostNewReply == nil) {
			return
		}
		assert.Equal(c, mattermostNewReply.RootId, mattermostNewMessage.Id)
	}, 10*time.Second, 500*time.Millisecond)
}

func TestSendMessageAndReplyToMattermostDirectMessage(t *testing.T) {
	setup(t)
	startTime := time.Now()
	generatedMessage := uuid.New().String()
	me, _, err := mmClient.GetMe(context.Background(), "")
	require.NoError(t, err)
	post := &model.Post{
		Message:   generatedMessage,
		ChannelId: testCfg.Mattermost.DmID,
		UserId:    me.Id,
	}
	newPost, _, err := mmClient.CreatePost(context.Background(), post)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		msTeamsMessages, err := msClient.ListChatMessages(testCfg.MSTeams.ChatID, startTime)
		require.NoError(t, err)

		var msteamsNewMessage *clientmodels.Message
		for _, msg := range msTeamsMessages {
			if strings.Contains(msg.Text, generatedMessage) {
				msteamsNewMessage = msg
			}
		}

		assert.NotNil(c, msteamsNewMessage)
	}, 10*time.Second, 500*time.Millisecond)

	generatedReply := uuid.New().String()
	replyPost := &model.Post{
		Message:   generatedReply,
		ChannelId: testCfg.Mattermost.DmID,
		UserId:    me.Id,
		RootId:    newPost.Id,
	}
	_, _, err = mmClient.CreatePost(context.Background(), replyPost)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		msTeamsMessages, err := msClient.ListChatMessages(testCfg.MSTeams.ChatID, startTime)
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

		assert.NotNil(c, msteamsNewMessage)
		assert.NotNil(c, msteamsNewReply)
		if (msteamsNewMessage == nil) || (msteamsNewReply == nil) {
			return
		}
		assert.Contains(c, msteamsNewReply.Text, msteamsNewMessage.ID)
	}, 10*time.Second, 500*time.Millisecond)
}
