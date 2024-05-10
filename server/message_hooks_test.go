package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	metricsmocks "github.com/mattermost/mattermost-plugin-msteams/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
)

func TestReactionHasBeenAdded(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	user2 := th.SetupUser(t, team)

	client1 := th.SetupClient(t, user1.Id)

	setupForChat := func(t *testing.T, sync bool) (*model.Channel, *model.Post, string, string) {
		t.Helper()
		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = sync
		})

		th.ConnectUser(t, user1.Id)
		th.ConnectUser(t, user2.Id)

		channel, _, err := client1.CreateDirectChannel(context.TODO(), user1.Id, user2.Id)
		require.Nil(t, err)

		var chatID, teamsMessageID string
		if sync {
			chatID = model.NewId()
			th.clientMock.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(&clientmodels.Chat{
				ID: chatID,
				Members: []clientmodels.ChatMember{
					{
						UserID:      "t" + user1.Id,
						Email:       user1.Email,
						DisplayName: user1.GetDisplayName(""),
					},
					{
						UserID:      "t" + user2.Id,
						Email:       user2.Email,
						DisplayName: user2.GetDisplayName(""),
					},
				},
			}, nil).Maybe()

			teamsMessageID = model.NewId()
			th.clientMock.On(
				"SendChat",
				chatID,
				mock.AnythingOfType("string"),
				(*clientmodels.Message)(nil),
				[]*clientmodels.Attachment(nil),
				[]models.ChatMessageMentionable{},
			).Return(&clientmodels.Message{
				ID:       teamsMessageID,
				CreateAt: time.Now(),
			}, nil).Times(1)
		}

		post, _, err := client1.CreatePost(context.TODO(), &model.Post{
			ChannelId: channel.Id,
			UserId:    user1.Id,
			Message:   "Test reaction",
		})
		require.Nil(t, err)

		if sync {
			assert.Eventually(t, func() bool {
				return th.getRelativeCounter(t,
					"msteams_connect_events_messages_total",
					withLabel("action", metrics.ActionCreated),
					withLabel("source", metrics.ActionSourceMattermost),
					withLabel("is_direct", "true"),
				) == 1
			}, 1*time.Second, 250*time.Millisecond)
		}

		return channel, post, chatID, teamsMessageID
	}

	t.Run("sync reactions disabled", func(t *testing.T) {
		th.Reset(t)
		channel, post, _, _ := setupForChat(t, true)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncReactions = false
		})

		_, _, err := client1.SaveReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionSetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("no corresponding Teams post", func(t *testing.T) {
		th.Reset(t)
		channel, post, _, _ := setupForChat(t, false)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = true
			c.SyncReactions = true
		})

		_, _, err := client1.SaveReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionSetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("direct message, sync disabled", func(t *testing.T) {
		th.Reset(t)
		channel, post, _, _ := setupForChat(t, true)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = false
		})

		_, _, err := client1.SaveReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionSetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("direct message, failed to set the reaction", func(t *testing.T) {
		th.Reset(t)
		channel, post, chatID, teamsMessageID := setupForChat(t, true)

		th.clientMock.On("SetChatReaction", chatID, teamsMessageID, "t"+user1.Id, "👍").Return(nil, fmt.Errorf("mock failure")).Times(1)

		_, _, err := client1.SaveReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionSetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("direct message, succeeded", func(t *testing.T) {
		th.Reset(t)
		channel, post, chatID, teamsMessageID := setupForChat(t, true)

		th.clientMock.On("SetChatReaction", chatID, teamsMessageID, "t"+user1.Id, "👍").Return(&clientmodels.Message{
			ID:           teamsMessageID,
			LastUpdateAt: time.Now(),
		}, nil).Times(1)

		_, _, err := client1.SaveReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Eventually(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionSetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) == 1
		}, 1*time.Second, 250*time.Millisecond)
	})

	setupForChannel := func(t *testing.T, sync bool) (*model.Channel, *model.Post, *storemodels.ChannelLink, string) {
		t.Helper()
		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncLinkedChannels = sync
		})

		th.ConnectUser(t, user1.Id)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		channelLink := th.LinkChannel(t, team, channel, user1)

		var teamsMessageID string
		if sync {
			teamsMessageID = model.NewId()
			th.clientMock.On(
				"SendMessageWithAttachments",
				channelLink.MSTeamsTeam,
				channelLink.MSTeamsChannel,
				"",
				mock.AnythingOfType("string"),
				[]*clientmodels.Attachment(nil),
				[]models.ChatMessageMentionable{},
			).Return(&clientmodels.Message{
				ID:       teamsMessageID,
				CreateAt: time.Now(),
			}, nil).Times(1)
		}

		post, _, err := client1.CreatePost(context.TODO(), &model.Post{
			ChannelId: channel.Id,
			UserId:    user1.Id,
			Message:   "Test reaction",
		})
		require.Nil(t, err)

		if sync {
			assert.Eventually(t, func() bool {
				return th.getRelativeCounter(t,
					"msteams_connect_events_messages_total",
					withLabel("action", metrics.ActionCreated),
					withLabel("source", metrics.ActionSourceMattermost),
					withLabel("is_direct", "false"),
				) == 1
			}, 1*time.Second, 250*time.Millisecond)
		}

		return channel, post, channelLink, teamsMessageID
	}

	t.Run("channel message, sync disabled", func(t *testing.T) {
		th.Reset(t)
		channel, post, _, _ := setupForChannel(t, false)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncReactions = true
		})

		_, _, err := client1.SaveReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionSetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "false"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("channel message, no longer linked", func(t *testing.T) {
		th.Reset(t)
		channel, post, _, _ := setupForChannel(t, true)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncReactions = true
		})

		// Unlink before setting reaction, but after sending the post.
		err := th.p.store.DeleteLinkByChannelID(channel.Id)
		require.NoError(t, err)

		_, _, err = client1.SaveReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionSetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "false"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("channel message, failed to set the reaction", func(t *testing.T) {
		th.Reset(t)
		channel, post, channelLink, teamsMessageID := setupForChannel(t, true)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncReactions = true
		})

		th.clientMock.On("SetReaction", channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, "", teamsMessageID, "t"+user1.Id, "👍").Return(nil, fmt.Errorf("mock failure")).Times(1)

		_, _, err := client1.SaveReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionSetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "false"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("channel message", func(t *testing.T) {
		th.Reset(t)
		channel, post, channelLink, teamsMessageID := setupForChannel(t, true)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncReactions = true
		})

		th.clientMock.On("SetReaction", channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, "", teamsMessageID, "t"+user1.Id, "👍").Return(&clientmodels.Message{
			ID:           teamsMessageID,
			LastUpdateAt: time.Now(),
		}, nil).Times(1)

		_, _, err := client1.SaveReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Eventually(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionSetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "false"),
			) == 1
		}, 1*time.Second, 250*time.Millisecond)
	})
}

func TestReactionHasBeenRemoved(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	user2 := th.SetupUser(t, team)

	client1 := th.SetupClient(t, user1.Id)

	setupForChat := func(t *testing.T, sync bool) (*model.Channel, *model.Post, string, string) {
		t.Helper()
		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncReactions = sync
			c.SyncDirectMessages = sync
		})

		th.ConnectUser(t, user1.Id)
		th.ConnectUser(t, user2.Id)

		var chatID, teamsMessageID string
		if sync {
			chatID = model.NewId()
			th.clientMock.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(&clientmodels.Chat{
				ID: chatID,
				Members: []clientmodels.ChatMember{
					{
						UserID:      "t" + user1.Id,
						Email:       user1.Email,
						DisplayName: user1.GetDisplayName(""),
					},
					{
						UserID:      "t" + user2.Id,
						Email:       user2.Email,
						DisplayName: user2.GetDisplayName(""),
					},
				},
			}, nil).Maybe()

			teamsMessageID = model.NewId()
			th.clientMock.On(
				"SendChat",
				chatID,
				mock.AnythingOfType("string"),
				(*clientmodels.Message)(nil),
				[]*clientmodels.Attachment(nil),
				[]models.ChatMessageMentionable{},
			).Return(&clientmodels.Message{
				ID:       teamsMessageID,
				CreateAt: time.Now(),
			}, nil).Times(1)
		}

		channel, _, err := client1.CreateDirectChannel(context.TODO(), user1.Id, user2.Id)
		require.Nil(t, err)

		post, _, err := client1.CreatePost(context.TODO(), &model.Post{
			ChannelId: channel.Id,
			UserId:    user1.Id,
			Message:   "Test reaction",
		})
		require.Nil(t, err)

		if sync {
			assert.Eventually(t, func() bool {
				return th.getRelativeCounter(t,
					"msteams_connect_events_messages_total",
					withLabel("action", metrics.ActionCreated),
					withLabel("source", metrics.ActionSourceMattermost),
					withLabel("is_direct", "true"),
				) == 1
			}, 5*time.Second, 250*time.Millisecond)
		}

		if sync {
			th.clientMock.On("SetChatReaction", chatID, teamsMessageID, "t"+user1.Id, "👍").Return(&clientmodels.Message{
				ID:           teamsMessageID,
				LastUpdateAt: time.Now(),
			}, nil).Times(1)
		}

		_, _, err = client1.SaveReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		// Wait for the reaction to sync, if enabled.
		if sync {
			assert.Eventually(t, func() bool {
				return th.getRelativeCounter(t,
					"msteams_connect_events_reactions_total",
					withLabel("action", metrics.ReactionSetAction),
					withLabel("source", metrics.ActionSourceMattermost),
					withLabel("is_direct", "true"),
				) == 1
			}, 1*time.Second, 250*time.Millisecond)
		}

		return channel, post, chatID, teamsMessageID
	}

	t.Run("sync reactions disabled", func(t *testing.T) {
		th.Reset(t)
		channel, post, _, _ := setupForChat(t, true)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncReactions = false
		})

		_, err := client1.DeleteReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionUnsetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("no corresponding Teams post", func(t *testing.T) {
		th.Reset(t)
		channel, post, _, _ := setupForChat(t, false)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = true
			c.SyncReactions = true
		})

		_, err := client1.DeleteReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionUnsetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("direct message, sync disabled", func(t *testing.T) {
		th.Reset(t)
		channel, post, _, _ := setupForChat(t, true)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = false
		})

		_, err := client1.DeleteReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionUnsetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("direct message, failed to unset the reaction", func(t *testing.T) {
		th.Reset(t)
		channel, post, chatID, teamsMessageID := setupForChat(t, true)

		th.clientMock.On("UnsetChatReaction", chatID, teamsMessageID, "t"+user1.Id, "👍").Return(nil, fmt.Errorf("mock failure")).Times(1)

		_, err := client1.DeleteReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionUnsetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("direct message, succeeded", func(t *testing.T) {
		th.Reset(t)
		channel, post, chatID, teamsMessageID := setupForChat(t, true)

		th.clientMock.On("UnsetChatReaction", chatID, teamsMessageID, "t"+user1.Id, "👍").Return(&clientmodels.Message{
			ID:           teamsMessageID,
			LastUpdateAt: time.Now(),
		}, nil).Times(1)

		_, err := client1.DeleteReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Eventually(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionUnsetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) == 1
		}, 1*time.Second, 250*time.Millisecond)
	})

	setupForChannel := func(t *testing.T, sync bool) (*model.Channel, *model.Post, *storemodels.ChannelLink, string) {
		t.Helper()

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncLinkedChannels = sync
		})

		th.ConnectUser(t, user1.Id)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))
		channelLink := th.LinkChannel(t, team, channel, user1)

		var teamsMessageID string
		if sync {
			teamsMessageID = model.NewId()
			th.clientMock.On(
				"SendMessageWithAttachments",
				channelLink.MSTeamsTeam,
				channelLink.MSTeamsChannel,
				"",
				mock.AnythingOfType("string"),
				[]*clientmodels.Attachment(nil),
				[]models.ChatMessageMentionable{},
			).Return(&clientmodels.Message{
				ID:       teamsMessageID,
				CreateAt: time.Now(),
			}, nil).Times(1)
		}

		post, _, err := client1.CreatePost(context.TODO(), &model.Post{
			ChannelId: channel.Id,
			UserId:    user1.Id,
			Message:   "Test reaction",
		})
		require.Nil(t, err)

		if sync {
			assert.Eventually(t, func() bool {
				return th.getRelativeCounter(t,
					"msteams_connect_events_messages_total",
					withLabel("action", metrics.ActionCreated),
					withLabel("source", metrics.ActionSourceMattermost),
					withLabel("is_direct", "false"),
				) == 1
			}, 1*time.Second, 250*time.Millisecond)
		}

		if sync {
			th.clientMock.On("SetReaction", channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, "", teamsMessageID, "t"+user1.Id, "👍").Return(&clientmodels.Message{
				ID:           teamsMessageID,
				LastUpdateAt: time.Now(),
			}, nil).Times(1)
		}

		_, _, err = client1.SaveReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		if sync {
			assert.Eventually(t, func() bool {
				return th.getRelativeCounter(t,
					"msteams_connect_events_reactions_total",
					withLabel("action", metrics.ReactionSetAction),
					withLabel("source", metrics.ActionSourceMattermost),
					withLabel("is_direct", "false"),
				) == 1
			}, 1*time.Second, 250*time.Millisecond)
		}

		return channel, post, channelLink, teamsMessageID
	}

	t.Run("channel message, sync disabled", func(t *testing.T) {
		th.Reset(t)
		channel, post, _, _ := setupForChannel(t, true)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncLinkedChannels = false
			c.SyncReactions = true
		})

		_, err := client1.DeleteReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionUnsetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "false"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("channel message, no longer linked", func(t *testing.T) {
		th.Reset(t)
		channel, post, _, _ := setupForChannel(t, true)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncReactions = true
			c.SyncLinkedChannels = true
		})

		err := th.p.store.DeleteLinkByChannelID(channel.Id)
		require.NoError(t, err)

		_, err = client1.DeleteReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionUnsetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "false"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("channel message, failed to set the reaction", func(t *testing.T) {
		th.Reset(t)
		channel, post, channelLink, teamsMessageID := setupForChannel(t, true)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncReactions = true
			c.SyncLinkedChannels = true
		})

		th.clientMock.On("UnsetReaction", channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, "", teamsMessageID, "t"+user1.Id, "👍").Return(nil, fmt.Errorf("mock failure")).Times(1)

		_, err := client1.DeleteReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionUnsetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "false"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("channel message", func(t *testing.T) {
		th.Reset(t)
		channel, post, channelLink, teamsMessageID := setupForChannel(t, true)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncReactions = true
			c.SyncLinkedChannels = true
		})

		th.clientMock.On("UnsetReaction", channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, "", teamsMessageID, "t"+user1.Id, "👍").Return(&clientmodels.Message{
			ID:           teamsMessageID,
			LastUpdateAt: time.Now(),
		}, nil).Times(1)

		_, err := client1.DeleteReaction(context.TODO(), &model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
			ChannelId: channel.Id,
		})
		require.Nil(t, err)

		assert.Eventually(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_reactions_total",
				withLabel("action", metrics.ReactionUnsetAction),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "false"),
			) == 1
		}, 1*time.Second, 250*time.Millisecond)
	})
}

func TestMessageHasBeenUpdated(t *testing.T) {
	mockChat := &clientmodels.Chat{
		ID: testutils.GetChatID(),
		Members: []clientmodels.ChatMember{
			{
				DisplayName: "mockDisplayName",
				UserID:      testutils.GetTeamsUserID(),
				Email:       testutils.GetTestEmail(),
			},
		},
	}
	mockChannelMessage := &clientmodels.Message{
		ID:        "mockMessageID",
		TeamID:    "mockTeamsTeamID",
		ChannelID: "mockTeamsChannelID",
	}
	mockChatMessage := &clientmodels.Message{
		ID:     "ms-teams-id",
		ChatID: testutils.GetChatID(),
	}
	for _, test := range []struct {
		Name         string
		SetupPlugin  func(*Plugin)
		SetupAPI     func(*plugintest.API)
		SetupStore   func(*storemocks.Store)
		SetupClient  func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics func(*metricsmocks.Metrics)
	}{
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncDirectMessages = true
				p.configuration.SyncLinkedChannels = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("mockChatID", nil).Times(2)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMsgID",
				}, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMsgID",
					MSTeamsChannel: testutils.GetChatID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("UpdateChatMessage", testutils.GetChatID(), "mockMsgID", "", []models.ChatMessageMentionable{}).Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and channel",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncDirectMessages = true
				p.configuration.SyncLinkedChannels = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
			},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and channel type is Open",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncDirectMessages = true
				p.configuration.SyncLinkedChannels = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeOpen), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
			},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and unable to get channel members",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncDirectMessages = true
				p.configuration.SyncLinkedChannels = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(nil, testutils.GetInternalServerAppError("unable to get channel members")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
			},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and unable to update the chat",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncDirectMessages = true
				p.configuration.SyncLinkedChannels = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(2)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get post info")).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "MessageHasBeenUpdated: Unable to get the link by channel ID and unable to create or get chat for users",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncDirectMessages = true
				p.configuration.SyncLinkedChannels = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(2)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(nil, errors.New("unable to create or get chat for users")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "false", "0", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "MessageHasBeenUpdated: Able to get the link by channel ID",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncDirectMessages = true
				p.configuration.SyncLinkedChannels = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MattermostTeamID:    "mockMattermostTeam",
					MattermostChannelID: "mockMattermostChannel",
					MSTeamsTeam:         "mockTeamsTeamID",
					MSTeamsChannel:      "mockTeamsChannelID",
				}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMessageID",
				}, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: "mockTeamsChannelID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", "mockTeamsChannelID", "", "mockMessageID", "", []models.ChatMessageMentionable{}).Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "MessageHasBeenUpdated: Able to get the link by channel ID but unable to update post",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncDirectMessages = true
				p.configuration.SyncLinkedChannels = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MattermostTeamID:    "mockMattermostTeamID",
					MattermostChannelID: "mockMattermostChannelID",
					MSTeamsTeam:         "mockTeamsTeamID",
					MSTeamsChannel:      "mockTeamsChannelID",
				}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockTeamsTeamID",
					MSTeamsChannel: "mockTeamsChannelID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", "mockTeamsChannelID", "", "", "", []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to update the post")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "false", "0", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "MessageHasBeenUpdated: Sync linked channels disabled",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncDirectMessages = true
				p.configuration.SyncLinkedChannels = false
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("GetLinkByChannelID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					MattermostTeamID:    "mockMattermostTeamID",
					MattermostChannelID: "mockMattermostChannelID",
					MSTeamsTeam:         "mockTeamsTeamID",
					MSTeamsChannel:      "mockTeamsChannelID",
				}, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
				}, nil).Times(2)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockTeamsTeamID",
					MSTeamsChannel: "mockTeamsChannelID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", "mockTeamsChannelID", "", "", "", []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to update the post")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "false", "0", mock.AnythingOfType("float64")).Once()
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			p := newTestPlugin(t)
			test.SetupPlugin(p)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			p.MessageHasBeenUpdated(&plugin.Context{}, testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()))
		})
	}
}

func TestSetChatReaction(t *testing.T) {
	mockChat := &clientmodels.Chat{
		ID: testutils.GetChatID(),
		Members: []clientmodels.ChatMember{
			{
				DisplayName: "mockDisplayName",
				UserID:      testutils.GetTeamsUserID(),
				Email:       testutils.GetTestEmail(),
			},
		},
	}
	mockChatMessage := &clientmodels.Message{
		ID:           "mockTeamsMessageID",
		ChatID:       testutils.GetChatID(),
		LastUpdateAt: testutils.GetMockTime(),
	}
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
		UpdateRequired  bool
	}{
		{
			Name:     "SetChatReaction: Unable to get the source user ID",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", testutils.GetInternalServerAppError("unable to get the source user ID")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the source user ID",
		},
		{
			Name:     "SetChatReaction: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "not connected user",
		},
		{
			Name: "SetChatReaction: Unable to get the chat ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the channel",
		},
		{
			Name: "SetChatReaction: Unable to set the chat reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("SetChatReaction", testutils.GetChatID(), "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(nil, errors.New("unable to set the chat reaction")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.SetChatReaction", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "unable to set the chat reaction",
			UpdateRequired:  true,
		},
		{
			Name: "SetChatReaction: Update not required on MS Teams",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", testutils.GetMockTime()).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), "mockTeamsMessageID").Return(mockChatMessage, nil).Once()
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "SetChatReaction: Unable to set the post last updateAt time",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", testutils.GetMockTime()).Return(errors.New("unable to set post lastUpdateAt value")).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("SetChatReaction", testutils.GetChatID(), "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionSetAction, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SetChatReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
		{
			Name: "SetChatReaction: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", testutils.GetMockTime()).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("SetChatReaction", testutils.GetChatID(), "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionSetAction, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SetChatReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			resp := p.SetChatReaction("mockTeamsMessageID", testutils.GetID(), testutils.GetChannelID(), "mockEmojiName", test.UpdateRequired)
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}

func TestSetReaction(t *testing.T) {
	mockChannelMessage := &clientmodels.Message{
		ID:           testutils.GetID(),
		TeamID:       "mockTeamsTeamID",
		ChannelID:    "mockTeamsChannelID",
		LastUpdateAt: testutils.GetMockTime(),
	}
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
	}{
		{
			Name:     "SetReaction: Unable to get the post info",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the post info",
		},
		{
			Name:     "SetReaction: Post info is nil",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "teams message not found",
		},
		{
			Name:     "SetReaction: Unable to get the client",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", mock.Anything).Return(nil, nil).Times(2)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "not connected user",
		},
		{
			Name: "SetReaction: Unable to set the reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(nil, errors.New("unable to set the reaction")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.SetReaction", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "unable to set the reaction",
		},
		{
			Name: "SetReaction: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", "", testutils.GetMockTime()).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionSetAction, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SetReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))

			resp := p.SetReaction("mockTeamsTeamID", "mockTeamsChannelID", testutils.GetUserID(), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), "mockName", true)
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}

func TestUnsetChatReaction(t *testing.T) {
	mockChat := &clientmodels.Chat{
		ID: testutils.GetChatID(),
		Members: []clientmodels.ChatMember{
			{
				DisplayName: "mockDisplayName",
				UserID:      testutils.GetTeamsUserID(),
				Email:       testutils.GetTestEmail(),
			},
		},
	}
	mockChatMessage := &clientmodels.Message{
		ID:           "mockTeamsMessageID",
		ChatID:       testutils.GetChatID(),
		LastUpdateAt: testutils.GetMockTime(),
	}
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
	}{
		{
			Name:     "UnsetChatReaction: Unable to get the source user ID",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", testutils.GetInternalServerAppError("unable to get the source user ID")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the source user ID",
		},
		{
			Name:     "UnsetChatReaction: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "not connected user",
		},
		{
			Name: "UnsetChatReaction: Unable to get the chat ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the channel",
		},
		{
			Name: "UnsetChatReaction: Unable to unset the chat reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("UnsetChatReaction", testutils.GetChatID(), "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(nil, errors.New("unable to unset the chat reaction")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetChatReaction", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "unable to unset the chat reaction",
		},
		{
			Name: "UnsetChatReaction: Unable to set the post last updateAt time",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", testutils.GetMockTime()).Return(errors.New("unable to set post lastUpdateAt value")).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("UnsetChatReaction", testutils.GetChatID(), "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetChatReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "UnsetChatReaction: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewString("/")}}, nil).Times(2)
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetChatID()+"mockTeamsMessageID", mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(2)
				store.On("SetPostLastUpdateAtByMSTeamsID", "mockTeamsMessageID", testutils.GetMockTime()).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("UnsetChatReaction", testutils.GetChatID(), "mockTeamsMessageID", testutils.GetID(), ":mockEmojiName:").Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetChatReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			resp := p.UnsetChatReaction("mockTeamsMessageID", testutils.GetID(), testutils.GetChannelID(), "mockEmojiName")
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}

func TestUnsetReaction(t *testing.T) {
	mockChannelMessage := &clientmodels.Message{
		ID:           testutils.GetID(),
		TeamID:       "mockTeamsTeamID",
		ChannelID:    "mockTeamsChannelID",
		LastUpdateAt: testutils.GetMockTime(),
	}
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
	}{
		{
			Name:     "UnsetReaction: Unable to get the post info",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the post info",
		},
		{
			Name:     "UnsetReaction: Post info is nil",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "teams message not found",
		},
		{
			Name:     "UnsetReaction: Unable to get the client",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", mock.Anything).Return(nil, nil).Times(2)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "not connected user",
		},
		{
			Name: "UnsetReaction: Unable to unset the reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(nil, errors.New("unable to unset the reaction")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetReaction", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "unable to unset the reaction",
		},
		{
			Name: "UnsetReaction: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", "", testutils.GetMockTime()).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))

			resp := p.UnsetReaction("mockTeamsTeamID", "mockTeamsChannelID", testutils.GetUserID(), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), "mockName")
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}

func TestSendChat(t *testing.T) {
	mockChat := &clientmodels.Chat{
		ID: testutils.GetChatID(),
		Members: []clientmodels.ChatMember{
			{
				DisplayName: "mockDisplayName",
				UserID:      testutils.GetTeamsUserID(),
				Email:       testutils.GetTestEmail(),
			},
		},
	}
	for _, test := range []struct {
		Name                     string
		SetupPlugin              func(*Plugin)
		SetupAPI                 func(*plugintest.API)
		SetupStore               func(*storemocks.Store)
		SetupClient              func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics             func(mockmetrics *metricsmocks.Metrics)
		ChatMembersSpanPlatforms bool
		ExpectedMessage          string
		ExpectedError            string
	}{
		{
			Name: "SendChat: Unable to get the source user ID, chat members don't span platforms",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", errors.New("unable to get the source user ID")).Times(1)
			},
			SetupClient:              func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:             func(mockmetrics *metricsmocks.Metrics) {},
			ChatMembersSpanPlatforms: false,
			ExpectedError:            "unable to get the source user ID",
		},
		{
			Name: "SendChat: Unable to get the source user ID, chat members span platforms",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), mock.Anything).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", errors.New("unable to get the source user ID")).Times(1)
			},
			SetupClient:              func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:             func(mockmetrics *metricsmocks.Metrics) {},
			ChatMembersSpanPlatforms: true,
			ExpectedError:            "unable to get the source user ID",
		},
		{
			Name: "SendChat: Unable to get the client, chat members don't span platforms",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:              func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:             func(mockmetrics *metricsmocks.Metrics) {},
			ChatMembersSpanPlatforms: false,
			ExpectedError:            "not connected user",
		},
		{
			Name: "SendChat: Unable to get the client, chat members span platforms",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), mock.Anything).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:              func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:             func(mockmetrics *metricsmocks.Metrics) {},
			ChatMembersSpanPlatforms: true,
			ExpectedError:            "not connected user",
		},
		{
			Name: "SendChat: Unable to create or get the chat",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(nil, errors.New("unable to create or get the chat")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError: "unable to create or get the chat",
		},
		{
			Name: "SendChat: Unable to send the chat",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), mockChat).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), []*clientmodels.Attachment{{
					ID: testutils.GetID(),
				}}, []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to send the chat")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError: "unable to send the chat",
		},
		{
			Name: "SendChat: Able to send the chat and not able to store the post",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(testutils.GetInternalServerAppError("unable to store the post")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), []*clientmodels.Attachment{{
					ID: testutils.GetID(),
				}}, []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), mockChat).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", true).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, true, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "SendChat: Unable to get the parent message",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(&storemodels.PostInfo{
					MSTeamsID: "mockParentMessageID",
				}, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Once()
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Once()
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), []*clientmodels.Attachment{{
					ID: testutils.GetID(),
				}}, []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), mockChat).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), "mockParentMessageID").Return(nil, errors.New("error in getting parent chat message")).Once()
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", true).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, true, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "SendChat: File attachments disabled",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = false
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
					return post.Message == "Attachments sent from Mattermost aren't yet delivered to Microsoft Teams."
				})).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), 0), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, true, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "SendChat: Unable to get the file info",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get file attachment")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, true).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, true, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "SendChat: Unable to get the file attachment from Mattermost",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the file attachment from Mattermost")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, true).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, true, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "SendChat: Unable to upload the attachments",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(&storemodels.PostInfo{
					MSTeamsID: "mockParentMessageID",
				}, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Once()
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Once()
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), mockChat).Return(nil, errors.New("unable to upload the attachments")).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), "mockParentMessageID").Return(&clientmodels.Message{
					ID:              "mockParentMessageID",
					UserID:          "mockUserID",
					Text:            "mockText",
					UserDisplayName: "mockUserDisplayName",
				}, nil).Once()
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", &clientmodels.Message{
					ID:              "mockParentMessageID",
					UserID:          "mockUserID",
					Text:            "mockText",
					UserDisplayName: "mockUserDisplayName",
				}, ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToUploadFileOnTeams, true).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, true, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "false", "0", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "SendChat: Valid",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", "mockRootID").Return(nil, nil).Once()
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChatID(),
					MSTeamsID:      "mockMessageID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(mockChat, nil).Times(1)
				uclient.On("GetChat", testutils.GetChatID()).Return(mockChat, nil).Times(1)
				uclient.On("UploadFile", "", "", "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), mockChat).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendChat", testutils.GetChatID(), "<p>mockMessage??????????</p>\n", (*clientmodels.Message)(nil), []*clientmodels.Attachment{{
					ID: testutils.GetID(),
				}}, []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", true).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, true, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendChat", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupPlugin(p)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())
			mockPost.Message = "mockMessage??????????"
			mockPost.RootId = "mockRootID"
			resp, err := p.SendChat(testutils.GetID(), []string{testutils.GetID(), testutils.GetID()}, mockPost, test.ChatMembersSpanPlatforms)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}

			assert.Equal(resp, test.ExpectedMessage)
		})
	}
}

func TestSend(t *testing.T) {
	for _, test := range []struct {
		Name            string
		SetupPlugin     func(*Plugin)
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
		ExpectedError   string
	}{
		{
			Name: "Send: Unable to get the client",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "Some users in this conversation rely on Microsoft Teams to receive your messages, but your account isn't connected. [Click here to connect your account](/plugins/com.mattermost.msteams-sync/connect).",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "Send: File attachments disabled",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = false
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
					return post.Message == "Attachments sent from Mattermost aren't yet delivered to Microsoft Teams."
				})).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), 0), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n", ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, false, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "Send: Unable to get the file info",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get file attachment")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n", ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, false, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "Send: Unable to get file attachment from Mattermost",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the file attachment from Mattermost")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n", ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, false, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "Send: Unable to send message with attachments",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UploadFile", testutils.GetID(), testutils.GetChannelID(), "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), (*clientmodels.Chat)(nil)).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n", []*clientmodels.Attachment{
					{
						ID: testutils.GetID(),
					},
				}, []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to send message with attachments")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, false, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError: "unable to send message with attachments",
		},
		{
			Name: "Send: Able to send message with attachments but unable to store posts",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(errors.New("unable to store posts")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UploadFile", testutils.GetID(), testutils.GetChannelID(), "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), (*clientmodels.Chat)(nil)).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n", []*clientmodels.Attachment{
					{
						ID: testutils.GetID(),
					},
				}, []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, false, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "Send: Able to send message with attachments with no error",
			SetupPlugin: func(p *Plugin) {
				p.configuration.SyncFileAttachments = true
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UploadFile", testutils.GetID(), testutils.GetChannelID(), "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), (*clientmodels.Chat)(nil)).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n", []*clientmodels.Attachment{
					{
						ID: testutils.GetID(),
					},
				}, []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, false, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupPlugin(p)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())
			mockPost.Message = "mockMessage??????????"
			resp, err := p.Send(testutils.GetID(), testutils.GetChannelID(), testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), mockPost)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}

			assert.Equal(resp, test.ExpectedMessage)
		})
	}
}

func TestDelete(t *testing.T) {
	for _, test := range []struct {
		Name          string
		SetupAPI      func(*plugintest.API)
		SetupStore    func(*storemocks.Store)
		SetupClient   func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics  func(mockmetrics *metricsmocks.Metrics)
		ExpectedError string
	}{
		{
			Name:     "Delete: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "Delete: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(metrics *metricsmocks.Metrics) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "Delete: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(metrics *metricsmocks.Metrics) {},
			ExpectedError: "post not found",
		},
		{
			Name: "Delete: Unable to delete the message",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("DeleteMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID").Return(errors.New("unable to delete the message")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteMessage", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError: "unable to delete the message",
		},
		{
			Name:     "Delete: Valid",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("DeleteMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID").Return(nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionDeleted, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			err := p.Delete("mockTeamsTeamID", testutils.GetChannelID(), testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()))
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

var (
	anyString      = mock.AnythingOfType("string")
	anyStringSlice = mock.AnythingOfType("[]string")
	anyInt         = mock.AnythingOfType("int")
	anyPost        = mock.AnythingOfType("*model.Post")
	anyFloat64     = mock.AnythingOfType("float64")
)

func TestDeleteChat(t *testing.T) {
	mockChat := &clientmodels.Chat{
		ID: testutils.GetChatID(),
		Members: []clientmodels.ChatMember{
			{
				DisplayName: "mockDisplayName",
				UserID:      testutils.GetTeamsUserID(),
				Email:       testutils.GetTestEmail(),
			},
		},
	}

	for _, test := range []struct {
		Name          string
		SetupAPI      func(*plugintest.API)
		SetupStore    func(*storemocks.Store)
		SetupClient   func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics  func(mockmetrics *metricsmocks.Metrics)
		ExpectedError string
	}{
		{
			Name: "DeleteChat: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", anyString, anyPost).Return(&model.Post{}).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "DeleteChat: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", anyString).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", anyString, anyInt, anyInt).Return(testutils.GetChannelMembers(10), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", anyString).Return(testutils.GetUserID(), nil)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", anyStringSlice).Return(mockChat, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", anyFloat64).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "true", "2XX", anyFloat64).Once()
			},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "DeleteChat: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", anyString).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", anyString, anyInt, anyInt).Return(testutils.GetChannelMembers(10), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", anyStringSlice).Return(mockChat, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", anyFloat64).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "true", "2XX", anyFloat64).Once()
			},
			ExpectedError: "post not found",
		},
		{
			Name: "DeleteChat: Unable to delete the message",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", anyString).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", anyString, anyInt, anyInt).Return(testutils.GetChannelMembers(10), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", anyStringSlice).Return(mockChat, nil).Times(1)
				uclient.On("DeleteChatMessage", anyString, anyString, "mockMSTeamsID").Return(errors.New("unable to delete the message")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", anyFloat64).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "true", "2XX", anyFloat64).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "false", "0", anyFloat64).Once()
			},
			ExpectedError: "unable to delete the message",
		},
		{
			Name: "DeleteChat: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", anyString).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", anyString, anyInt, anyInt).Return(testutils.GetChannelMembers(10), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", anyStringSlice).Return(mockChat, nil).Times(1)
				uclient.On("DeleteChatMessage", anyString, anyString, "mockMSTeamsID").Return(nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", anyFloat64).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "true", "2XX", anyFloat64).Once()
				mockmetrics.On("ObserveMessage", metrics.ActionDeleted, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			err := p.DeleteChat(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()))
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	mockChannelMessage := &clientmodels.Message{
		ID:        "mockMSTeamsID",
		TeamID:    "mockTeamsTeamID",
		ChannelID: testutils.GetChannelID(),
	}
	for _, test := range []struct {
		Name           string
		SetupAPI       func(*plugintest.API)
		SetupStore     func(*storemocks.Store)
		SetupClient    func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics   func(mockmetrics *metricsmocks.Metrics)
		ExpectedError  string
		UpdateRequired bool
	}{
		{
			Name:     "Update: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "Update: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "Update: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "post not found",
		},
		{
			Name: "Update: Unable to update the message",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to update the message")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError:  "unable to update the message",
			UpdateRequired: true,
		},
		{
			Name: "Update: Update not required on MS Teams",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChannelID(),
					MSTeamsID:      "mockMSTeamsID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("GetMessage", "mockTeamsTeamID", testutils.GetChannelID(), "mockMSTeamsID").Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "Update: Unable to store the link posts",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChannelID(),
					MSTeamsID:      "mockMSTeamsID",
				}).Return(errors.New("unable to store the link posts")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
		{
			Name: "Update: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChannelID(),
					MSTeamsID:      "mockMSTeamsID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())
			mockPost.Message = "mockMessage??????????"
			err := p.Update("mockTeamsTeamID", testutils.GetChannelID(), testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), mockPost, test.UpdateRequired)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestUpdateChat(t *testing.T) {
	mockChatMessage := &clientmodels.Message{
		ID:     "mockChatID",
		ChatID: "mockTeamsTeamID",
	}
	for _, test := range []struct {
		Name           string
		SetupAPI       func(*plugintest.API)
		SetupStore     func(*storemocks.Store)
		SetupClient    func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics   func(mockmetrics *metricsmocks.Metrics)
		ExpectedError  string
		UpdateRequired bool
	}{
		{
			Name: "UpdateChat: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "UpdateChat: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "post not found",
		},
		{
			Name:     "UpdateChat: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},

			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "UpdateChat: Unable to update the message",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to update the message")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateChatMessage", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError:  "unable to update the message",
			UpdateRequired: true,
		},
		{
			Name:     "UpdateChat: Update not required on MS Teams",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockTeamsTeamID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("GetChatMessage", "mockChatID", "mockTeamsTeamID").Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "UpdateChat: Unable to store the link posts",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockTeamsTeamID",
				}).Return(errors.New("unable to store the link posts")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
		{
			Name: "UpdateChat: Valid",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockTeamsTeamID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())
			mockPost.Message = "mockMessage??????????"
			err := p.UpdateChat("mockChatID", testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), mockPost, test.UpdateRequired)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestGetChatIDForChannel(t *testing.T) {
	mockChat := &clientmodels.Chat{
		ID: testutils.GetChatID(),
		Members: []clientmodels.ChatMember{
			{
				DisplayName: "mockDisplayName",
				UserID:      testutils.GetTeamsUserID(),
				Email:       testutils.GetTestEmail(),
			},
		},
	}
	for _, test := range []struct {
		Name           string
		SetupAPI       func(*plugintest.API)
		SetupStore     func(*storemocks.Store)
		SetupClient    func(*clientmocks.Client)
		ExpectedError  string
		ExpectedResult string
	}{
		{
			Name: "GetChatIDForChannel: Unable to get the channel",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore:    func(store *storemocks.Store) {},
			SetupClient:   func(client *clientmocks.Client) {},
			ExpectedError: "unable to get the channel",
		},
		{
			Name: "GetChatIDForChannel: Channel type is 'open'",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeOpen), nil).Times(1)
			},
			SetupStore:    func(store *storemocks.Store) {},
			SetupClient:   func(client *clientmocks.Client) {},
			ExpectedError: "invalid channel type, chatID is only available for direct messages and group messages",
		},
		{
			Name: "GetChatIDForChannel: Unable to get the channel members",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(nil, testutils.GetInternalServerAppError("unable to get the channel members")).Times(1)
			},
			SetupStore:    func(store *storemocks.Store) {},
			SetupClient:   func(client *clientmocks.Client) {},
			ExpectedError: "unable to get the channel members",
		},
		{
			Name: "GetChatIDForChannel: Unable to store users",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", errors.New("unable to store the user")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client) {},
			ExpectedError: "unable to store the user",
		},
		{
			Name: "GetChatIDForChannel: Unable to create or get chat for users",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("mockTeamsUserID", nil).Times(2)
				store.On("GetTokenForMattermostUser", "mockClientUserID").Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("CreateOrGetChatForUsers", []string{"mockTeamsUserID", "mockTeamsUserID"}).Return(nil, errors.New("unable to create or get chat for users")).Times(1)
			},
			ExpectedError: "unable to create or get chat for users",
		},
		{
			Name: "GetChatIDForChannel: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("mockTeamsUserID", nil).Times(2)
				store.On("GetTokenForMattermostUser", "mockClientUserID").Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("CreateOrGetChatForUsers", []string{"mockTeamsUserID", "mockTeamsUserID"}).Return(mockChat, nil).Times(1)
			},
			ExpectedResult: testutils.GetChatID(),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			client := p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client)
			test.SetupClient(client)
			resp, err := p.GetChatIDForChannel(client, testutils.GetChannelID())
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
				assert.Equal(resp, "")
			} else {
				assert.Nil(err)
				assert.Equal(resp, test.ExpectedResult)
			}
		})
	}
}

func TestGetMentionsData(t *testing.T) {
	for _, test := range []struct {
		Name                  string
		Message               string
		ChatID                string
		SetupAPI              func(*plugintest.API)
		SetupStore            func(*storemocks.Store)
		SetupClient           func(*clientmocks.Client)
		ExpectedMessage       string
		ExpectedMentionsCount int
	}{
		{
			Name:            "GetMentionsData: mentioned in direct chat message",
			Message:         "Hi @all",
			ExpectedMessage: "Hi @all",
			ChatID:          testutils.GetChatID(),
			SetupAPI:        func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{}, nil)
			},
			SetupStore: func(store *storemocks.Store) {},
		},
		{
			Name:            "GetMentionsData: mentioned all in a group chat message",
			Message:         "Hi @all",
			ExpectedMessage: "Hi <at id=\"0\">Everyone</at>",
			ChatID:          testutils.GetChatID(),
			SetupAPI:        func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					Type: "G",
				}, nil)
			},
			SetupStore:            func(store *storemocks.Store) {},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: error occurred while getting chat",
			Message:         "Hi @all",
			ExpectedMessage: "Hi <at id=\"0\">@all</at>",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(nil, errors.New("error occurred while getting chat"))
			},
			SetupStore:            func(store *storemocks.Store) {},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: mentioned all in Teams channel",
			Message:         "Hi @all",
			ExpectedMessage: "Hi <at id=\"0\">mock-name</at>",
			SetupAPI:        func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChannelInTeam", testutils.GetTeamID(), testutils.GetChannelID()).Return(&clientmodels.Channel{
					DisplayName: "mock-name",
				}, nil)
			},
			SetupStore:            func(store *storemocks.Store) {},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: error occurred while getting the MS Teams channel",
			Message:         "Hi @all",
			ExpectedMessage: "Hi <at id=\"0\">@all</at>",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChannelInTeam", testutils.GetTeamID(), testutils.GetChannelID()).Return(nil, errors.New("error occurred while getting the MS Teams channel"))
			},
			SetupStore:            func(store *storemocks.Store) {},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: mentioned a user",
			Message:         "Hi @test-username",
			ExpectedMessage: "Hi <at id=\"0\">mock-name</at>",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByUsername", "test-username").Return(testutils.GetUser("mock-role", "mock-email"), nil)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetUser", testutils.GetUserID()).Return(&clientmodels.User{
					DisplayName: "mock-name",
				}, nil)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetUserID(), nil)
			},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: mentioned all and a specific user in a group chat",
			Message:         "Hi @all @test-username",
			ExpectedMessage: "Hi <at id=\"0\">Everyone</at> <at id=\"1\">mock-name</at>",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByUsername", "test-username").Return(testutils.GetUser("mock-role", "mock-email"), nil)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					Type: "G",
				}, nil)
				client.On("GetUser", testutils.GetUserID()).Return(&clientmodels.User{
					DisplayName: "mock-name",
				}, nil)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetUserID(), nil)
			},
			ExpectedMentionsCount: 2,
		},
		{
			Name:            "GetMentionsData: error getting MM user with username",
			Message:         "Hi @test-username",
			ExpectedMessage: "Hi @test-username",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByUsername", "test-username").Return(nil, testutils.GetInternalServerAppError("error getting MM user with username"))
			},
			SetupClient: func(client *clientmocks.Client) {},
			SetupStore:  func(store *storemocks.Store) {},
		},
		{
			Name:            "GetMentionsData: error getting msteams user ID from MM user ID",
			Message:         "Hi @test-username",
			ExpectedMessage: "Hi @test-username",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByUsername", "test-username").Return(testutils.GetUser("mock-role", "mock-email"), nil)
			},
			SetupClient: func(client *clientmocks.Client) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", errors.New("error getting msteams user ID from MM user ID"))
			},
		},
		{
			Name:            "GetMentionsData: error getting msteams user",
			Message:         "Hi @test-username",
			ExpectedMessage: "Hi @test-username",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByUsername", "test-username").Return(testutils.GetUser("mock-role", "mock-email"), nil)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetUser", testutils.GetUserID()).Return(nil, errors.New("error getting msteams user"))
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetUserID(), nil)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))

			client := p.msteamsAppClient.(*clientmocks.Client)
			test.SetupClient(client)

			msg, mentions := p.getMentionsData(test.Message, testutils.GetTeamID(), testutils.GetChannelID(), test.ChatID, client)
			assert.Equal(test.ExpectedMessage, msg)
			assert.Equal(test.ExpectedMentionsCount, len(mentions))
		})
	}
}

func TestUserWillLogin(t *testing.T) {
	for _, test := range []struct {
		Name                               string
		User                               *model.User
		UserIsRemote                       bool // the remote id is set by the plugin during the test
		AutomaticallyPromoteSyntheticUsers bool
		SetupAPI                           func(*plugintest.API)
		SetupStore                         func(*storemocks.Store)
		SetupClient                        func(*clientmocks.Client)
		Result                             string
	}{
		{
			Name: "Autopromotion works",
			User: &model.User{
				Id: testutils.GetID(),
			},
			UserIsRemote:                       true,
			AutomaticallyPromoteSyntheticUsers: true,
			SetupAPI: func(api *plugintest.API) {
				api.On("UpdateUser", mock.MatchedBy(func(user *model.User) bool {
					return !user.IsRemote()
				})).Once().Return(nil, nil)
				api.On("LogInfo", "Promoted synthetic user", "user_id", testutils.GetID()).Once()
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{}, nil)
			},
			SetupStore: func(store *storemocks.Store) {},
			Result:     "",
		},
		{
			Name: "UpdateUser failed during autopromotion",
			User: &model.User{
				Id: testutils.GetID(),
			},
			UserIsRemote:                       true,
			AutomaticallyPromoteSyntheticUsers: true,
			SetupAPI: func(api *plugintest.API) {
				api.On("UpdateUser", mock.MatchedBy(func(user *model.User) bool {
					return !user.IsRemote()
				})).Once().Return(nil, model.NewAppError("UpdateUser", "err from test", nil, "", 500))
				api.On("LogWarn", "Failed to promote synthetic user", "user_id", testutils.GetID(), "err", "err from test").Once()
			},
			SetupClient: func(client *clientmocks.Client) {},
			SetupStore:  func(store *storemocks.Store) {},
			Result:      "Unable to promote synthetic user",
		},
		{
			Name: "No autopromotion",
			User: &model.User{
				Id: testutils.GetID(),
			},
			UserIsRemote:                       true,
			AutomaticallyPromoteSyntheticUsers: false,
			SetupAPI:                           func(api *plugintest.API) {},
			SetupClient:                        func(client *clientmocks.Client) {},
			SetupStore:                         func(store *storemocks.Store) {},
			Result:                             "",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			cfg := p.configuration
			cfg.AutomaticallyPromoteSyntheticUsers = test.AutomaticallyPromoteSyntheticUsers

			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))

			client := p.msteamsAppClient.(*clientmocks.Client)
			test.SetupClient(client)

			user := test.User
			if test.UserIsRemote {
				user.RemoteId = &p.remoteID
			}

			res := p.UserWillLogIn(nil, user)

			assert.Equal(test.Result, res)
		})
	}
}
