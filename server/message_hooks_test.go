package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
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

	t.Run("no corresponding Teams post", func(t *testing.T) {
		th.Reset(t)
		channel, post, _, _ := setupForChat(t, false)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = true
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

	t.Run("no corresponding Teams post", func(t *testing.T) {
		th.Reset(t)
		channel, post, _, _ := setupForChat(t, false)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = true
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

func TestMessageHasBeenPosted(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	expectChat := func(th *testHelper, t *testing.T, users ...*model.User) {
		t.Helper()

		var members []clientmodels.ChatMember
		for _, user := range users {
			members = append(members, clientmodels.ChatMember{
				UserID:      "t" + user.Id,
				Email:       user.Email,
				DisplayName: user.GetDisplayName(""),
			})
		}

		chatID := model.NewId()
		th.clientMock.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(&clientmodels.Chat{
			ID:      chatID,
			Members: members,
		}, nil).Maybe()

		teamsMessageID := model.NewId()
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

	expectChatSync := func(t *testing.T) {
		t.Helper()

		assert.Eventually(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_messages_total",
				withLabel("action", metrics.ActionCreated),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) == 1
		}, 1*time.Second, 250*time.Millisecond)
	}

	expectNoChatSync := func(t *testing.T) {
		t.Helper()

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_messages_total",
				withLabel("action", metrics.ActionCreated),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) == 1
		}, 1*time.Second, 250*time.Millisecond)
	}

	t.Run("MS Teams bot ignored", func(t *testing.T) {
		t.Skip("Not yet implemented")
	})

	t.Run("system message ignored", func(t *testing.T) {
		t.Skip("Not yet implemented")
	})

	t.Run("chat", func(t *testing.T) {
		type parameters struct {
			SenderConnected    bool
			SelectiveSync      bool
			SyncDirectMessages bool
			SyncGroupMessages  bool
			SyncLinkedChannels bool
		}
		runPermutations(t, parameters{}, func(t *testing.T, params parameters) {
			th.Reset(t)

			th.setPluginConfiguration(t, func(c *configuration) {
				c.SelectiveSync = params.SelectiveSync
				c.SyncDirectMessages = params.SyncDirectMessages
				c.SyncGroupMessages = params.SyncGroupMessages
				c.SyncLinkedChannels = params.SyncLinkedChannels
			})

			t.Run("dm with self", func(t *testing.T) {
				th.Reset(t)

				user1 := th.SetupUser(t, team)

				client1 := th.SetupClient(t, user1.Id)

				channel, _, err := client1.CreateDirectChannel(context.TODO(), user1.Id, user1.Id)
				require.NoError(t, err)

				if params.SenderConnected {
					th.ConnectUser(t, user1.Id)
				}

				if params.SenderConnected && params.SyncDirectMessages && !params.SelectiveSync {
					expectChat(th, t, user1, user1)
				}

				_, _, err = client1.CreatePost(context.TODO(), &model.Post{
					ChannelId: channel.Id,
					UserId:    user1.Id,
					Message:   "Test message",
				})
				require.NoError(t, err)

				if params.SenderConnected && params.SyncDirectMessages && !params.SelectiveSync {
					expectChatSync(t)
				} else {
					expectNoChatSync(t)
				}
			})

			t.Run("dm with remote user", func(t *testing.T) {
				th.Reset(t)

				user1 := th.SetupUser(t, team)
				user2 := th.SetupRemoteUser(t, team)

				client1 := th.SetupClient(t, user1.Id)

				channel, _, err := client1.CreateDirectChannel(context.TODO(), user1.Id, user2.Id)
				require.NoError(t, err)

				if params.SenderConnected {
					th.ConnectUser(t, user1.Id)
				}

				if params.SenderConnected && params.SyncDirectMessages {
					expectChat(th, t, user1, user2)
				}

				_, _, err = client1.CreatePost(context.TODO(), &model.Post{
					ChannelId: channel.Id,
					UserId:    user1.Id,
					Message:   "Test message",
				})
				require.NoError(t, err)

				if params.SenderConnected && params.SyncDirectMessages {
					expectChatSync(t)
				} else {
					expectNoChatSync(t)
				}
			})

			t.Run("dm with local user", func(t *testing.T) {
				th.Reset(t)

				user1 := th.SetupUser(t, team)
				user2 := th.SetupUser(t, team)

				client1 := th.SetupClient(t, user1.Id)

				channel, _, err := client1.CreateDirectChannel(context.TODO(), user1.Id, user2.Id)
				require.NoError(t, err)

				if params.SenderConnected {
					th.ConnectUser(t, user1.Id)
				}

				_, _, err = client1.CreatePost(context.TODO(), &model.Post{
					ChannelId: channel.Id,
					UserId:    user1.Id,
					Message:   "Test message",
				})
				require.NoError(t, err)

				expectNoChatSync(t)
			})

			t.Run("gm with only remote users", func(t *testing.T) {
				th.Reset(t)

				user1 := th.SetupUser(t, team)
				user2 := th.SetupRemoteUser(t, team)
				user3 := th.SetupRemoteUser(t, team)

				client1 := th.SetupClient(t, user1.Id)

				channel, _, err := client1.CreateGroupChannel(context.TODO(), []string{user1.Id, user2.Id, user3.Id})
				require.NoError(t, err)

				if params.SenderConnected {
					th.ConnectUser(t, user1.Id)
				}

				if params.SenderConnected && params.SyncGroupMessages {
					expectChat(th, t, user1, user2, user3)
				}

				_, _, err = client1.CreatePost(context.TODO(), &model.Post{
					ChannelId: channel.Id,
					UserId:    user1.Id,
					Message:   "Test message",
				})
				require.NoError(t, err)

				if params.SenderConnected && params.SyncGroupMessages {
					expectChatSync(t)
				} else {
					expectNoChatSync(t)
				}
			})

			t.Run("gm with only local users", func(t *testing.T) {
				th.Reset(t)

				user1 := th.SetupUser(t, team)
				user2 := th.SetupUser(t, team)
				user3 := th.SetupUser(t, team)

				client1 := th.SetupClient(t, user1.Id)

				channel, _, err := client1.CreateGroupChannel(context.TODO(), []string{user1.Id, user2.Id, user3.Id})
				require.NoError(t, err)

				if params.SenderConnected {
					th.ConnectUser(t, user1.Id)
				}

				_, _, err = client1.CreatePost(context.TODO(), &model.Post{
					ChannelId: channel.Id,
					UserId:    user1.Id,
					Message:   "Test message",
				})
				require.NoError(t, err)

				expectNoChatSync(t)
			})

			t.Run("gm with local and remote users", func(t *testing.T) {
				th.Reset(t)

				user1 := th.SetupUser(t, team)
				user2 := th.SetupUser(t, team)
				user3 := th.SetupRemoteUser(t, team)

				client1 := th.SetupClient(t, user1.Id)

				channel, _, err := client1.CreateGroupChannel(context.TODO(), []string{user1.Id, user2.Id, user3.Id})
				require.NoError(t, err)

				if params.SenderConnected {
					th.ConnectUser(t, user1.Id)
				}

				// Unconditionally link user2 with their Teams account. This is actually
				// a current shortcoming with GMs in that we can't even start a conversation
				// until we know the mapping for all users involved.
				err = th.p.store.SetUserInfo(user2.Id, "t"+user2.Id, nil)
				require.NoError(t, err)

				if params.SenderConnected && params.SyncGroupMessages && !params.SelectiveSync {
					expectChat(th, t, user1, user2, user3)
				}

				_, _, err = client1.CreatePost(context.TODO(), &model.Post{
					ChannelId: channel.Id,
					UserId:    user1.Id,
					Message:   "Test message",
				})
				require.NoError(t, err)

				if params.SenderConnected && params.SyncGroupMessages && !params.SelectiveSync {
					expectChatSync(t)
				} else {
					expectNoChatSync(t)
				}
			})
		})
	})

	t.Run("channel post", func(t *testing.T) {
		th.Reset(t)
		t.Skip("Not yet implemented")
	})
}

func TestMessageHasBeenUpdated(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)
	user1 := th.SetupUser(t, team)
	user2 := th.SetupUser(t, team)
	user3 := th.SetupUser(t, team)

	client1 := th.SetupClient(t, user1.Id)
	th.SetupWebsocketClientForUser(t, user1.Id,
		model.WebsocketEventPosted,
		model.WebsocketEventMultipleChannelsViewed,
		model.WebsocketEventPostEdited,
		model.WebsocketEventDirectAdded,
		model.WebsocketEventGroupAdded,
	)

	makeChatMembers := func(users ...*model.User) []clientmodels.ChatMember {
		var chatMembers []clientmodels.ChatMember
		for _, user := range users {
			chatMembers = append(chatMembers, clientmodels.ChatMember{
				UserID:      "t" + user.Id,
				Email:       user.Email,
				DisplayName: user1.GetDisplayName(""),
			})
		}

		return chatMembers
	}

	setupChat := func(th *testHelper, t *testing.T, users ...*model.User) (*model.Post, string, string) {
		var channel *model.Channel
		var err error
		if len(users) == 0 {
			t.Fatalf("cannot setup chat for %d users", len(users))
		} else if len(users) == 1 {
			channel, _, err = client1.CreateDirectChannel(context.TODO(), users[0].Id, users[0].Id)
			require.Nil(t, err)
		} else if len(users) == 2 {
			channel, _, err = client1.CreateDirectChannel(context.TODO(), users[0].Id, users[1].Id)
			require.Nil(t, err)
		} else if len(users) <= 8 {
			var userIDs []string
			for _, user := range users {
				userIDs = append(userIDs, user.Id)
			}

			channel, _, err = client1.CreateGroupChannel(context.TODO(), userIDs)
			require.Nil(t, err)
		} else if len(users) > 8 {
			t.Fatalf("cannot setup chat for %d users", len(users))
		}

		senderUser := users[0]

		var chatID, teamsMessageID string
		if th.p.getConfiguration().SyncDirectMessages {
			chatID = model.NewId()
			th.clientMock.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(&clientmodels.Chat{
				ID:      chatID,
				Members: makeChatMembers(users...),
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
			UserId:    senderUser.Id,
			Message:   "Test reaction",
		})
		require.NoError(t, err)

		if th.p.getConfiguration().SyncDirectMessages {
			assert.Eventually(t, func() bool {
				return th.getRelativeCounter(t,
					"msteams_connect_events_messages_total",
					withLabel("action", metrics.ActionCreated),
					withLabel("source", metrics.ActionSourceMattermost),
					withLabel("is_direct", "true"),
				) == 1
			}, 1*time.Second, 250*time.Millisecond)
		}

		return post, chatID, teamsMessageID
	}

	t.Run("direct message updated, no corresponding chat", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = false
			c.SelectiveSync = false
		})

		th.ConnectUser(t, user1.Id)
		th.ConnectUser(t, user2.Id)

		post, _, _ := setupChat(th, t, user1, user2)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = true
		})

		th.clientMock.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(nil, errors.New("not found")).Once()

		post.Message = "Test updated"
		_, _, err := client1.UpdatePost(context.TODO(), post.Id, post)
		require.NoError(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_messages_total",
				withLabel("action", metrics.ActionUpdated),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("direct message updated, no corresponding post", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = false
			c.SelectiveSync = false
		})

		th.ConnectUser(t, user1.Id)
		th.ConnectUser(t, user2.Id)

		post, _, _ := setupChat(th, t, user1, user2)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = true
		})

		chatID := model.NewId()
		th.clientMock.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(&clientmodels.Chat{
			ID:      chatID,
			Members: makeChatMembers(user1, user2),
		}, nil).Maybe()

		post.Message = "Test updated"
		_, _, err := client1.UpdatePost(context.TODO(), post.Id, post)
		require.NoError(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_messages_total",
				withLabel("action", metrics.ActionUpdated),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("direct message updated, direct message sync disabled when update occurs", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = true
			c.SelectiveSync = false
		})

		th.ConnectUser(t, user1.Id)
		th.ConnectUser(t, user2.Id)

		post, _, _ := setupChat(th, t, user1, user2)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = false
			c.SelectiveSync = false
		})

		post.Message = "Test updated"
		_, _, err := client1.UpdatePost(context.TODO(), post.Id, post)
		require.NoError(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_messages_total",
				withLabel("action", metrics.ActionUpdated),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("direct message updated, disconnected when update occurs", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = true
			c.SelectiveSync = false
		})

		th.ConnectUser(t, user1.Id)
		th.ConnectUser(t, user2.Id)

		post, _, _ := setupChat(th, t, user1, user2)

		th.DisconnectUser(t, user1.Id)

		post.Message = "Test updated"
		_, _, err := client1.UpdatePost(context.TODO(), post.Id, post)
		require.NoError(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_messages_total",
				withLabel("action", metrics.ActionUpdated),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("direct message updated successfully", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = true
			c.SelectiveSync = false
		})

		th.ConnectUser(t, user1.Id)
		th.ConnectUser(t, user2.Id)

		post, chatID, teamsMessageID := setupChat(th, t, user1, user2)

		post.Message = "Test updated"
		_, _, err := client1.UpdatePost(context.TODO(), post.Id, post)
		require.NoError(t, err)

		th.clientMock.On(
			"UpdateChatMessage",
			chatID,
			teamsMessageID,
			"<p>Test updated</p>\n",
			[]models.ChatMessageMentionable{},
		).Return(&clientmodels.Message{
			ID:           teamsMessageID,
			CreateAt:     time.Now(),
			LastUpdateAt: time.Now(),
		}, nil).Times(1)

		assert.Eventually(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_messages_total",
				withLabel("action", metrics.ActionUpdated),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) == 1
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("group message updated, no corresponding chat", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = false // used to signal setupChat for now
			c.SyncGroupMessages = false
			c.SelectiveSync = false
		})

		th.ConnectUser(t, user1.Id)
		th.ConnectUser(t, user2.Id)
		th.ConnectUser(t, user3.Id)

		post, _, _ := setupChat(th, t, user1, user2, user3)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncGroupMessages = true
		})

		th.clientMock.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(nil, errors.New("not found")).Once()

		post.Message = "Test updated"
		_, _, err := client1.UpdatePost(context.TODO(), post.Id, post)
		require.NoError(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_messages_total",
				withLabel("action", metrics.ActionUpdated),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("group message updated, no corresponding post", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = false // used to signal setupChat for now
			c.SyncGroupMessages = false
			c.SelectiveSync = false
		})

		th.ConnectUser(t, user1.Id)
		th.ConnectUser(t, user2.Id)
		th.ConnectUser(t, user3.Id)

		post, _, _ := setupChat(th, t, user1, user2, user3)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncGroupMessages = true
		})

		chatID := model.NewId()
		th.clientMock.On("CreateOrGetChatForUsers", mock.AnythingOfType("[]string")).Return(&clientmodels.Chat{
			ID:      chatID,
			Members: makeChatMembers(user1, user2, user3),
		}, nil).Maybe()

		post.Message = "Test updated"
		_, _, err := client1.UpdatePost(context.TODO(), post.Id, post)
		require.NoError(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_messages_total",
				withLabel("action", metrics.ActionUpdated),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("group message updated, group message sync disabled when update occurs", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = true // used to signal setupChat for now
			c.SyncGroupMessages = true
			c.SelectiveSync = false
		})

		th.ConnectUser(t, user1.Id)
		th.ConnectUser(t, user2.Id)

		post, _, _ := setupChat(th, t, user1, user2)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = false // used to signal setupChat for now
			c.SyncGroupMessages = false
			c.SelectiveSync = false
		})

		post.Message = "Test updated"
		_, _, err := client1.UpdatePost(context.TODO(), post.Id, post)
		require.NoError(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_messages_total",
				withLabel("action", metrics.ActionUpdated),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("group message updated, disconnected when update occurs", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = true // used to signal setupChat for now
			c.SyncGroupMessages = true
			c.SelectiveSync = false
		})

		th.ConnectUser(t, user1.Id)
		th.ConnectUser(t, user2.Id)

		post, _, _ := setupChat(th, t, user1, user2)

		th.DisconnectUser(t, user1.Id)

		post.Message = "Test updated"
		_, _, err := client1.UpdatePost(context.TODO(), post.Id, post)
		require.NoError(t, err)

		assert.Never(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_messages_total",
				withLabel("action", metrics.ActionUpdated),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) > 0
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("group message updated successfully", func(t *testing.T) {
		th.Reset(t)

		th.setPluginConfiguration(t, func(c *configuration) {
			c.SyncDirectMessages = true // used to signal setupChat for now
			c.SyncGroupMessages = true
			c.SelectiveSync = false
		})

		th.ConnectUser(t, user1.Id)
		th.ConnectUser(t, user2.Id)
		th.ConnectUser(t, user3.Id)

		post, chatID, teamsMessageID := setupChat(th, t, user1, user2, user3)

		post.Message = "Test updated"
		_, _, err := client1.UpdatePost(context.TODO(), post.Id, post)
		require.NoError(t, err)

		th.clientMock.On(
			"UpdateChatMessage",
			chatID,
			teamsMessageID,
			"<p>Test updated</p>\n",
			[]models.ChatMessageMentionable{},
		).Return(&clientmodels.Message{
			ID:           teamsMessageID,
			CreateAt:     time.Now(),
			LastUpdateAt: time.Now(),
		}, nil).Times(1)

		assert.Eventually(t, func() bool {
			return th.getRelativeCounter(t,
				"msteams_connect_events_messages_total",
				withLabel("action", metrics.ActionUpdated),
				withLabel("source", metrics.ActionSourceMattermost),
				withLabel("is_direct", "true"),
			) == 1
		}, 1*time.Second, 250*time.Millisecond)
	})

	t.Run("channel message updated, channel unlinked", func(t *testing.T) {
		t.Skip("Not yet implemented")
	})

	t.Run("channel message updated, no corresponding post", func(t *testing.T) {
		t.Skip("Not yet implemented")
	})

	t.Run("channel message updated successfully", func(t *testing.T) {
		t.Skip("Not yet implemented")
	})
}

func TestMessageHasBeenDeleted(t *testing.T) {
	t.Skip("Not yet implemented")
}
