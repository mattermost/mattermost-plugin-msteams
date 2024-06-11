package main

import (
	"database/sql"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestSetChatReaction(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	setupChat := func(t *testing.T, emojiName string) (*model.User, *model.Channel, string, string) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)

		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		channel, err := th.p.apiClient.Channel.GetDirect(senderUser.Id, user2.Id)
		require.NoError(t, err)

		chatID := model.NewId()
		messageID := model.NewId()
		mockTeamsHelper := newMockTeamsHelper(th)
		mockTeamsHelper.registerChat(chatID, []*model.User{senderUser, user2})
		mockTeamsHelper.registerChatMessage(chatID, messageID, senderUser, "message")

		return senderUser, channel, chatID, messageID
	}

	t.Run("sender not connected", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, _, messageID := setupChat(t, emojiName)

		updateRequired := true
		err := th.p.SetChatReaction(messageID, senderUser.Id, channel.Id, emojiName, updateRequired)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("failed to set the chat reaction", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, chatID, messageID := setupChat(t, emojiName)
		th.ConnectUser(t, senderUser.Id)

		updateRequired := true

		th.clientMock.On("SetChatReaction", chatID, messageID, "t"+senderUser.Id, "🎉").Return(nil, errors.New("unable to set the chat reaction"))

		err := th.p.SetChatReaction(messageID, senderUser.Id, channel.Id, emojiName, updateRequired)
		require.Error(t, err)
	})

	t.Run("chat reaction added", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, chatID, messageID := setupChat(t, emojiName)

		th.ConnectUser(t, senderUser.Id)

		updateRequired := true

		th.clientMock.On("SetChatReaction", chatID, messageID, "t"+senderUser.Id, "🎉").Return(&clientmodels.Message{
			LastUpdateAt: time.Now(),
		}, nil).Once()

		err := th.p.SetChatReaction(messageID, senderUser.Id, channel.Id, emojiName, updateRequired)
		require.NoError(t, err)
	})

	t.Run("chat reaction added, update not required", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, _, messageID := setupChat(t, emojiName)

		th.ConnectUser(t, senderUser.Id)

		updateRequired := false

		err := th.p.SetChatReaction(messageID, senderUser.Id, channel.Id, emojiName, updateRequired)
		require.NoError(t, err)
	})
}

func TestSetReaction(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	setup := func(t *testing.T, emojiName string, linkPost bool) (*model.User, *storemodels.ChannelLink, *model.Post, storemodels.PostInfo) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)

		channel := th.SetupPublicChannel(t, team, WithMembers(senderUser))
		channelLink := th.LinkChannel(t, team, channel, senderUser)

		post := &model.Post{
			UserId:    senderUser.Id,
			ChannelId: channel.Id,
			Message:   "test set reaction",
		}
		err := th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		var postInfo storemodels.PostInfo
		if linkPost {
			postInfo = storemodels.PostInfo{
				MattermostID:        post.Id,
				MSTeamsChannel:      model.NewId(),
				MSTeamsID:           model.NewId(),
				MSTeamsLastUpdateAt: time.Now(),
			}
			err = th.p.GetStore().LinkPosts(postInfo)
			require.NoError(t, err)

			mockTeamsHelper := newMockTeamsHelper(th)
			mockTeamsHelper.registerMessage(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, postInfo.MSTeamsID, senderUser, post.Message)
		}

		return senderUser, channelLink, post, postInfo
	}

	t.Run("sender not connected", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, _ := setup(t, emojiName, true)

		updateRequired := true
		err := th.p.SetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName, updateRequired)
		require.ErrorContains(t, err, "not connected user")
	})

	t.Run("post not linked", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, _ := setup(t, emojiName, false)

		err := th.p.UnsetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("failed to set the reaction", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, postInfo := setup(t, emojiName, true)
		th.ConnectUser(t, senderUser.Id)

		th.clientMock.On("SetReaction", channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, "", postInfo.MSTeamsID, "t"+senderUser.Id, "🎉").Return(nil, errors.New("unable to set the reaction"))

		updateRequired := true
		err := th.p.SetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName, updateRequired)
		require.Error(t, err)
	})

	t.Run("reaction added", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, postInfo := setup(t, emojiName, true)

		th.ConnectUser(t, senderUser.Id)

		th.clientMock.On("SetReaction", channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, "", postInfo.MSTeamsID, "t"+senderUser.Id, "🎉").Return(&clientmodels.Message{
			LastUpdateAt: time.Now(),
		}, nil).Once()

		updateRequired := true
		err := th.p.SetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName, updateRequired)
		require.NoError(t, err)
	})

	t.Run("reaction added, update not required", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, _ := setup(t, emojiName, true)

		th.ConnectUser(t, senderUser.Id)

		updateRequired := false
		err := th.p.SetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName, updateRequired)
		require.NoError(t, err)
	})
}

func TestUnsetChatReaction(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	setupChat := func(t *testing.T, emojiName string) (*model.User, *model.Channel, string, string) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)

		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		channel, err := th.p.apiClient.Channel.GetDirect(senderUser.Id, user2.Id)
		require.NoError(t, err)

		chatID := model.NewId()
		messageID := model.NewId()
		mockTeamsHelper := newMockTeamsHelper(th)
		mockTeamsHelper.registerChat(chatID, []*model.User{senderUser, user2})
		mockTeamsHelper.registerChatMessage(chatID, messageID, senderUser, "message")

		return senderUser, channel, chatID, messageID
	}

	t.Run("sender not connected", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, messageID, _ := setupChat(t, emojiName)

		err := th.p.UnsetChatReaction(messageID, senderUser.Id, channel.Id, emojiName)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("failed to set the chat reaction", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, chatID, messageID := setupChat(t, emojiName)
		th.ConnectUser(t, senderUser.Id)

		th.clientMock.On("UnsetChatReaction", chatID, messageID, "t"+senderUser.Id, "🎉").Return(nil, errors.New("unable to unset the chat reaction"))

		err := th.p.UnsetChatReaction(messageID, senderUser.Id, channel.Id, emojiName)
		require.Error(t, err)
	})

	t.Run("chat reaction removed", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, chatID, messageID := setupChat(t, emojiName)
		th.ConnectUser(t, senderUser.Id)

		th.clientMock.On("UnsetChatReaction", chatID, messageID, "t"+senderUser.Id, "🎉").Return(&clientmodels.Message{
			LastUpdateAt: time.Now(),
		}, nil).Once()

		err := th.p.UnsetChatReaction(messageID, senderUser.Id, channel.Id, emojiName)
		require.NoError(t, err)
	})
}

func TestUnsetReaction(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	setup := func(t *testing.T, emojiName string, linkPost bool) (*model.User, *storemodels.ChannelLink, *model.Post, storemodels.PostInfo) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)

		channel := th.SetupPublicChannel(t, team, WithMembers(senderUser))
		channelLink := th.LinkChannel(t, team, channel, senderUser)

		post := &model.Post{
			UserId:    senderUser.Id,
			ChannelId: channel.Id,
			Message:   "test set reaction",
		}
		err := th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		var postInfo storemodels.PostInfo
		if linkPost {
			postInfo = storemodels.PostInfo{
				MattermostID:        post.Id,
				MSTeamsChannel:      model.NewId(),
				MSTeamsID:           model.NewId(),
				MSTeamsLastUpdateAt: time.Now(),
			}
			err = th.p.GetStore().LinkPosts(postInfo)
			require.NoError(t, err)

			mockTeamsHelper := newMockTeamsHelper(th)
			mockTeamsHelper.registerMessage(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, postInfo.MSTeamsID, senderUser, post.Message)
		}

		return senderUser, channelLink, post, postInfo
	}

	t.Run("sender not connected", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, _ := setup(t, emojiName, true)

		err := th.p.UnsetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName)
		require.ErrorContains(t, err, "not connected user")
	})

	t.Run("post not linked", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, _ := setup(t, emojiName, false)

		err := th.p.UnsetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("failed to unset the reaction", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, postInfo := setup(t, emojiName, true)
		th.ConnectUser(t, senderUser.Id)

		th.clientMock.On("UnsetReaction", channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, "", postInfo.MSTeamsID, "t"+senderUser.Id, "🎉").Return(nil, errors.New("unable to set the reaction"))

		err := th.p.UnsetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName)
		require.Error(t, err)
	})

	t.Run("reaction removed", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, postInfo := setup(t, emojiName, true)

		th.ConnectUser(t, senderUser.Id)

		th.clientMock.On("UnsetReaction", channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, "", postInfo.MSTeamsID, "t"+senderUser.Id, "🎉").Return(&clientmodels.Message{
			LastUpdateAt: time.Now(),
		}, nil).Once()

		err := th.p.UnsetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName)
		require.NoError(t, err)
	})
}
