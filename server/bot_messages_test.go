package main

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func TestSendEphemeralConnectMessage(t *testing.T) {
	t.Skip()
}

func TestSendConnectMessage(t *testing.T) {
	t.Skip()
}

func TestSendWelcomeMessageWithNotificationAction(t *testing.T) {
	th := setupTestHelper(t)

	// Arrange
	team := th.SetupTeam(t)
	user := th.SetupUser(t, team)

	// Act
	err := th.p.SendWelcomeMessageWithNotificationAction(user.Id)
	require.NoError(t, err)

	// Assert
	dc, err := th.p.apiClient.Channel.GetDirect(user.Id, th.p.botUserID)
	require.NoError(t, err)
	posts, err := th.p.apiClient.Post.GetPostsSince(dc.Id, time.Now().Add(-1*time.Minute).UnixMilli())
	require.NoError(t, err)
	require.Len(t, posts.Order, 1)

	post := posts.Posts[posts.Order[0]]
	// make sure we have the message and a button
	require.Contains(t, post.Message, "**Get notified for MS Teams Chats**")
	require.Len(t, post.Attachments(), 1)
	require.Len(t, post.Attachments()[0].Actions, 2)
	require.EqualValues(t, model.PostActionTypeButton, post.Attachments()[0].Actions[0].Type)
	require.EqualValues(t, "Enable notifications", post.Attachments()[0].Actions[0].Name)
	require.False(t, post.Attachments()[0].Actions[0].Disabled)
	require.EqualValues(t, model.PostActionTypeButton, post.Attachments()[0].Actions[1].Type)
	require.EqualValues(t, "Disable", post.Attachments()[0].Actions[1].Name)
	require.False(t, post.Attachments()[0].Actions[1].Disabled)
}
