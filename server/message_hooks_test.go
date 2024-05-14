package main

import (
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
