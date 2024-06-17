package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCreatedActivity(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("unable to get original message", func(t *testing.T) {
		th.Reset(t)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID: "invalid_chat_id",
		}

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(nil, errors.New("Error while getting original chat")).Times(1)

		discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonUnableToGetTeamsData, discardReason)
	})

	t.Run("nil message", func(t *testing.T) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)
		th.ConnectUser(t, senderUser.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    "chat_id",
			MessageID: "message_id",
		}

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + senderUser.Id,
				},
				{
					UserID: "t" + user1.Id,
				},
			},
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(nil, nil).Times(1)

		discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonUnableToGetTeamsData, discardReason)
	})

	t.Run("skipping not user event", func(t *testing.T) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)
		th.ConnectUser(t, senderUser.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    "chat_id",
			MessageID: "message_id",
		}

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + senderUser.Id,
				},
				{
					UserID: "t" + user1.Id,
				},
			},
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(&clientmodels.Message{}, nil).Times(1)

		discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonNotUserEvent, discardReason)
	})

	t.Run("known duplicate post", func(t *testing.T) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)
		th.ConnectUser(t, senderUser.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    "chat_id",
			MessageID: "message_id",
		}
		now := time.Now()
		postID := model.NewId()
		teamsChannelID := ""

		// Simulate the post having originated from Mattermost. Later, we'll let the code
		// do this itself once.
		err := th.p.GetStore().LinkPosts(storemodels.PostInfo{
			MattermostID:        postID,
			MSTeamsID:           activityIds.MessageID,
			MSTeamsChannel:      fmt.Sprintf(activityIds.ChatID + teamsChannelID),
			MSTeamsLastUpdateAt: now,
		})
		require.NoError(t, err)

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + senderUser.Id,
				},
				{
					UserID: "t" + user1.Id,
				},
			},
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(
			&clientmodels.Message{
				ID:              activityIds.MessageID,
				UserID:          "t" + senderUser.Id,
				ChatID:          activityIds.ChatID,
				UserDisplayName: senderUser.GetDisplayName(model.ShowFullName),
				Text:            "message",
				CreateAt:        now,
				LastUpdateAt:    now,
			}, nil).Times(1)

		discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonDuplicatedPost, discardReason)
	})

	t.Run("discovered duplicate post", func(t *testing.T) {
		th.Reset(t)
		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.SyncChats = true
		})

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    "chat_id",
			MessageID: "message_id",
		}
		now := time.Now()

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + user1.Id,
				},
				{
					UserID: "t" + user2.Id,
				},
			},
			Type: "D",
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(
			&clientmodels.Message{
				ID:              activityIds.MessageID,
				UserID:          "t" + user2.Id,
				ChatID:          activityIds.ChatID,
				UserDisplayName: user2.GetDisplayName(model.ShowFullName),
				Text:            "message<abbr title=\"generated-from-mattermost\"></abbr>",
				CreateAt:        now,
				LastUpdateAt:    now,
			}, nil).Times(1)

		discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonGeneratedFromMattermost, discardReason)
	})

	t.Run("skipping messages from bot user", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		senderBotUser, err := th.p.apiClient.User.Get(th.p.botUserID)
		require.NoError(t, err)
		th.ConnectUser(t, senderBotUser.Id)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    "chat_id",
			MessageID: "message_id",
		}
		now := time.Now()

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + user1.Id,
				},
				{
					UserID: "t" + senderBotUser.Id,
				},
			},
			Type: "D",
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(
			&clientmodels.Message{
				ID:              activityIds.MessageID,
				UserID:          "t" + senderBotUser.Id,
				ChatID:          activityIds.ChatID,
				UserDisplayName: senderBotUser.GetDisplayName(model.ShowFullName),
				Text:            "message",
				CreateAt:        now,
				LastUpdateAt:    now,
			}, nil).Times(1)

		discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonIsBotUser, discardReason)
	})

	t.Run("chats disabled", func(t *testing.T) {
		th.Reset(t)
		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.SyncChats = false
		})

		senderUser := th.SetupUser(t, team)
		th.ConnectUser(t, senderUser.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    "chat_id",
			MessageID: "message_id",
		}
		now := time.Now()

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + user1.Id,
				},
				{
					UserID: "t" + senderUser.Id,
				},
			},
			Type: "D",
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(
			&clientmodels.Message{
				ID:              activityIds.MessageID,
				UserID:          "t" + senderUser.Id,
				ChatID:          activityIds.ChatID,
				UserDisplayName: senderUser.GetDisplayName(model.ShowFullName),
				Text:            "message",
				CreateAt:        now,
				LastUpdateAt:    now,
			}, nil).Times(1)
		th.appClientMock.On("GetUser", "t"+senderUser.Id).Return(&clientmodels.User{
			ID: "t" + senderUser.Id,
		}, nil).Once()

		discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
	})

	t.Run("group chats disabled", func(t *testing.T) {
		th.Reset(t)
		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.SyncChats = false
		})

		senderUser := th.SetupUser(t, team)
		th.ConnectUser(t, senderUser.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    "chat_id",
			MessageID: "message_id",
		}
		now := time.Now()

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + user1.Id,
				},
				{
					UserID: "t" + senderUser.Id,
				},
				{
					UserID: "t" + user2.Id,
				},
			},
			Type: "G",
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(
			&clientmodels.Message{
				ID:              activityIds.MessageID,
				UserID:          "t" + senderUser.Id,
				ChatID:          activityIds.ChatID,
				UserDisplayName: senderUser.GetDisplayName(model.ShowFullName),
				Text:            "message",
				CreateAt:        now,
				LastUpdateAt:    now,
			}, nil).Times(1)
		th.appClientMock.On("GetUser", "t"+senderUser.Id).Return(&clientmodels.User{
			ID: "t" + senderUser.Id,
		}, nil).Once()

		discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
	})

	t.Run("unable to get user", func(t *testing.T) {
		th.Reset(t)
		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.SyncChats = true
		})

		senderUser := th.SetupUser(t, team)
		th.ConnectUser(t, senderUser.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    "chat_id",
			MessageID: "message_id",
		}
		now := time.Now()

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + user1.Id,
				},
				{
					UserID: "t" + senderUser.Id,
				},
			},
			Type: "D",
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(
			&clientmodels.Message{
				ID:              activityIds.MessageID,
				UserID:          "t" + senderUser.Id,
				ChatID:          activityIds.ChatID,
				UserDisplayName: senderUser.GetDisplayName(model.ShowFullName),
				Text:            "message",
				CreateAt:        now,
				LastUpdateAt:    now,
			}, nil).Times(1)
		th.appClientMock.On("GetUser", "t"+senderUser.Id).Return(&clientmodels.User{
			ID: "t" + senderUser.Id,
		}, nil).Twice()
		th.appClientMock.On("GetUser", "t"+user1.Id).Return(nil, fmt.Errorf("unable to get user")).Once()

		discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonOther, discardReason)
	})

	t.Run("chats", func(t *testing.T) {
		type parameters struct {
			SyncChats         bool
			SyncNotifications bool
		}
		runPermutations(t, parameters{}, func(t *testing.T, params parameters) {
			th.Reset(t)

			th.setPluginConfigurationTemporarily(t, func(c *configuration) {
				c.SyncChats = params.SyncChats
				c.SyncNotifications = params.SyncNotifications
			})

			t.Run("chat message", func(t *testing.T) {
				th.Reset(t)

				senderUser := th.SetupUser(t, team)
				th.ConnectUser(t, senderUser.Id)

				user1 := th.SetupUser(t, team)
				th.ConnectUser(t, user1.Id)

				msg := (*clientmodels.Message)(nil)
				subscriptionID := "test"
				activityIds := clientmodels.ActivityIds{
					ChatID:    "chat_id",
					MessageID: "message_id",
				}

				mockTeams := newMockTeamsHelper(th)
				mockTeams.registerChat(activityIds.ChatID, []*model.User{user1, senderUser})
				mockTeams.registerChatMessage(activityIds.ChatID, activityIds.MessageID, senderUser, "message")

				discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
				if params.SyncChats && !params.SyncNotifications {
					assert.Equal(t, metrics.DiscardedReasonNone, discardReason)
					th.assertDMFromUser(t, senderUser.Id, user1.Id, "message")
				} else {
					if params.SyncNotifications {
						assert.Equal(t, metrics.DiscardedReasonNone, discardReason)
					} else {
						assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
					}

					th.assertNoDMFromUser(t, senderUser.Id, user1.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
				}
			})

			t.Run("group chat message", func(t *testing.T) {
				th.Reset(t)

				senderUser := th.SetupUser(t, team)
				th.ConnectUser(t, senderUser.Id)

				user1 := th.SetupUser(t, team)
				th.ConnectUser(t, user1.Id)

				user2 := th.SetupUser(t, team)
				th.ConnectUser(t, user2.Id)

				msg := (*clientmodels.Message)(nil)
				subscriptionID := "test"
				activityIds := clientmodels.ActivityIds{
					ChatID:    "chat_id",
					MessageID: "message_id",
				}

				mockTeams := newMockTeamsHelper(th)
				mockTeams.registerGroupChat(activityIds.ChatID, []*model.User{user1, user2, senderUser})
				mockTeams.registerChatMessage(activityIds.ChatID, activityIds.MessageID, senderUser, "message")

				discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
				if params.SyncChats && !params.SyncNotifications {
					assert.Equal(t, metrics.DiscardedReasonNone, discardReason)
					th.assertGMFromUsers(t, senderUser.Id, []string{user1.Id, user2.Id}, "message")
				} else {
					if params.SyncNotifications {
						assert.Equal(t, metrics.DiscardedReasonNone, discardReason)
					} else {
						assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
					}

					th.assertNoGMFromUsers(t, senderUser.Id, []string{user1.Id, user2.Id}, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
				}
			})
		})
	})

	t.Run("channels", func(t *testing.T) {
		type parameters struct {
			SyncLinkedChannels bool
			SyncNotifications  bool
		}
		runPermutations(t, parameters{}, func(t *testing.T, params parameters) {
			th.Reset(t)

			th.setPluginConfigurationTemporarily(t, func(c *configuration) {
				c.SyncLinkedChannels = params.SyncLinkedChannels
				c.SyncNotifications = params.SyncNotifications
			})

			t.Run("linked channel", func(t *testing.T) {
				th.Reset(t)

				senderUser := th.SetupUser(t, team)
				th.ConnectUser(t, senderUser.Id)

				user1 := th.SetupUser(t, team)
				th.ConnectUser(t, user1.Id)

				user2 := th.SetupUser(t, team)
				th.ConnectUser(t, user2.Id)

				channel := th.SetupPublicChannel(t, team, WithMembers(user1))
				_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
				require.Nil(t, appErr)

				channelLink := storemodels.ChannelLink{
					MattermostTeamID:    team.Id,
					MattermostChannelID: channel.Id,
					MSTeamsTeam:         model.NewId(),
					MSTeamsChannel:      model.NewId(),
					Creator:             user1.Id,
				}

				err := th.p.store.StoreChannelLink(&channelLink)
				require.NoError(t, err)

				msg := (*clientmodels.Message)(nil)
				subscriptionID := "test"
				activityIds := clientmodels.ActivityIds{
					TeamID:    channelLink.MSTeamsTeam,
					ChannelID: channelLink.MSTeamsChannel,
					MessageID: "message_id",
				}

				mockTeams := newMockTeamsHelper(th)
				mockTeams.registerMessage(activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID, senderUser, "message")

				discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)

				if params.SyncLinkedChannels {
					assert.Equal(t, metrics.DiscardedReasonNone, discardReason)
					th.assertPostInChannel(t, senderUser.Id, channel.Id, "message")
				} else {
					assert.Equal(t, metrics.DiscardedReasonLinkedChannelsDisabled, discardReason)
					th.assertNoPostInChannel(t, senderUser.Id, channel.Id)
				}
			})
		})
	})

	t.Run("notifications", func(t *testing.T) {
		type parameters struct {
			SyncChats         bool
			SyncNotifications bool
			NotificationPref  bool
		}
		runPermutations(t, parameters{}, func(t *testing.T, params parameters) {
			th.Reset(t)

			th.setPluginConfigurationTemporarily(t, func(c *configuration) {
				c.SyncChats = params.SyncChats
				c.SyncNotifications = params.SyncNotifications
			})

			t.Run("chat message", func(t *testing.T) {
				th.Reset(t)

				senderUser := th.SetupUser(t, team)
				th.ConnectUser(t, senderUser.Id)

				user1 := th.SetupUser(t, team)
				th.ConnectUser(t, user1.Id)

				err := th.p.setNotificationPreference(user1.Id, params.NotificationPref)
				require.NoError(t, err)

				botUser, err := th.p.apiClient.User.Get(th.p.botUserID)
				require.NoError(t, err)
				th.ConnectUser(t, botUser.Id)

				msg := (*clientmodels.Message)(nil)
				subscriptionID := "test"
				activityIds := clientmodels.ActivityIds{
					ChatID:    "chat_id",
					MessageID: "message_id",
				}

				mockTeams := newMockTeamsHelper(th)
				mockTeams.registerChat(activityIds.ChatID, []*model.User{user1, senderUser})
				mockTeams.registerChatMessage(activityIds.ChatID, activityIds.MessageID, senderUser, "message")

				discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
				if params.SyncNotifications {
					assert.Equal(t, metrics.DiscardedReasonNone, discardReason)
				} else {
					if params.SyncChats {
						assert.Equal(t, metrics.DiscardedReasonNone, discardReason)
					} else {
						assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
					}
				}

				if params.SyncNotifications && params.NotificationPref {
					th.assertDMFromUserRe(t, botUser.Id, user1.Id, "message")
				} else {
					th.assertNoDMFromUser(t, botUser.Id, user1.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
				}
			})

			t.Run("group chat message", func(t *testing.T) {
				th.Reset(t)

				senderUser := th.SetupUser(t, team)
				th.ConnectUser(t, senderUser.Id)

				user1 := th.SetupUser(t, team)
				th.ConnectUser(t, user1.Id)

				err := th.p.setNotificationPreference(user1.Id, params.NotificationPref)
				require.NoError(t, err)

				user2 := th.SetupUser(t, team)
				th.ConnectUser(t, user2.Id)

				botUser, err := th.p.apiClient.User.Get(th.p.botUserID)
				require.NoError(t, err)
				th.ConnectUser(t, botUser.Id)

				msg := (*clientmodels.Message)(nil)
				subscriptionID := "test"
				activityIds := clientmodels.ActivityIds{
					ChatID:    "chat_id",
					MessageID: "message_id",
				}

				mockTeams := newMockTeamsHelper(th)
				mockTeams.registerGroupChat(activityIds.ChatID, []*model.User{user1, user2, senderUser})
				mockTeams.registerChatMessage(activityIds.ChatID, activityIds.MessageID, senderUser, "message")

				discardReason := th.p.activityHandler.handleCreatedActivity(msg, subscriptionID, activityIds)
				if params.SyncNotifications {
					assert.Equal(t, metrics.DiscardedReasonNone, discardReason)
				} else {
					if params.SyncChats {
						assert.Equal(t, metrics.DiscardedReasonNone, discardReason)
					} else {
						assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
					}
				}

				if params.SyncNotifications && params.NotificationPref {
					th.assertDMFromUserRe(t, botUser.Id, user1.Id, "message")
				} else {
					th.assertNoDMFromUser(t, botUser.Id, user1.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
				}
			})
		})
	})
}

func TestHandleUpdatedActivity(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("unable to get original message", func(t *testing.T) {
		th.Reset(t)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID: "invalid_chat_id",
		}

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(nil, errors.New("Error while getting original chat")).Times(1)

		discardReason := th.p.activityHandler.handleUpdatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonUnableToGetTeamsData, discardReason)
	})

	t.Run("nil message", func(t *testing.T) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)
		th.ConnectUser(t, senderUser.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    "chat_id",
			MessageID: "message_id",
		}

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + senderUser.Id,
				},
				{
					UserID: "t" + user1.Id,
				},
			},
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(nil, nil).Times(1)

		discardReason := th.p.activityHandler.handleUpdatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonUnableToGetTeamsData, discardReason)
	})

	t.Run("skipping not user event", func(t *testing.T) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)
		th.ConnectUser(t, senderUser.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    "chat_id",
			MessageID: "message_id",
		}

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + senderUser.Id,
				},
				{
					UserID: "t" + user1.Id,
				},
			},
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(&clientmodels.Message{}, nil).Times(1)

		discardReason := th.p.activityHandler.handleUpdatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonNotUserEvent, discardReason)
	})

	t.Run("unknown post", func(t *testing.T) {
		t.Skip("not yet implemented")
	})

	t.Run("known duplicate post", func(t *testing.T) {
		t.Skip("not yet implemented")
	})

	t.Run("skipping messages from bot user", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		senderBotUser, err := th.p.apiClient.User.Get(th.p.botUserID)
		require.NoError(t, err)
		th.ConnectUser(t, senderBotUser.Id)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    "chat_id",
			MessageID: "message_id",
		}
		now := time.Now()

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + user1.Id,
				},
				{
					UserID: "t" + senderBotUser.Id,
				},
			},
			Type: "D",
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(
			&clientmodels.Message{
				ID:              activityIds.MessageID,
				UserID:          "t" + senderBotUser.Id,
				ChatID:          activityIds.ChatID,
				UserDisplayName: senderBotUser.GetDisplayName(model.ShowFullName),
				Text:            "message",
				CreateAt:        now,
				LastUpdateAt:    now,
			}, nil).Times(1)

		discardReason := th.p.activityHandler.handleUpdatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonIsBotUser, discardReason)
	})

	t.Run("chats disabled", func(t *testing.T) {
		th.Reset(t)
		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.SyncChats = false
		})

		senderUser := th.SetupUser(t, team)
		th.ConnectUser(t, senderUser.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		channel, err := th.p.apiClient.Channel.GetDirect(senderUser.Id, user1.Id)
		require.NoError(t, err)

		post := &model.Post{
			UserId:    senderUser.Id,
			ChannelId: channel.Id,
			Message:   "message",
		}
		err = th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		// Simulate the post having originated from Mattermost. Later, we'll let the code
		// do this itself once.
		messageID := model.NewId()
		chatID := model.NewId()
		postInfo := storemodels.PostInfo{
			MattermostID:        post.Id,
			MSTeamsID:           messageID,
			MSTeamsChannel:      chatID,
			MSTeamsLastUpdateAt: time.Now(),
		}
		err = th.p.GetStore().LinkPosts(postInfo)
		require.NoError(t, err)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    chatID,
			MessageID: messageID,
		}
		now := time.Now()

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + user1.Id,
				},
				{
					UserID: "t" + senderUser.Id,
				},
			},
			Type: "D",
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(
			&clientmodels.Message{
				ID:              activityIds.MessageID,
				UserID:          "t" + senderUser.Id,
				ChatID:          activityIds.ChatID,
				UserDisplayName: senderUser.GetDisplayName(model.ShowFullName),
				Text:            "message",
				CreateAt:        now,
				LastUpdateAt:    now,
			}, nil).Times(1)

		discardReason := th.p.activityHandler.handleUpdatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
	})

	t.Run("group chats disabled", func(t *testing.T) {
		th.Reset(t)
		th.setPluginConfigurationTemporarily(t, func(c *configuration) {
			c.SyncChats = false
		})

		senderUser := th.SetupUser(t, team)
		th.ConnectUser(t, senderUser.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		channel, err := th.p.apiClient.Channel.GetDirect(senderUser.Id, user1.Id)
		require.NoError(t, err)

		post := &model.Post{
			UserId:    senderUser.Id,
			ChannelId: channel.Id,
			Message:   "message",
		}
		err = th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		// Simulate the post having originated from Mattermost. Later, we'll let the code
		// do this itself once.
		messageID := model.NewId()
		chatID := model.NewId()
		postInfo := storemodels.PostInfo{
			MattermostID:        post.Id,
			MSTeamsID:           messageID,
			MSTeamsChannel:      chatID,
			MSTeamsLastUpdateAt: time.Now(),
		}
		err = th.p.GetStore().LinkPosts(postInfo)
		require.NoError(t, err)

		msg := (*clientmodels.Message)(nil)
		subscriptionID := "test"
		activityIds := clientmodels.ActivityIds{
			ChatID:    chatID,
			MessageID: messageID,
		}
		now := time.Now()

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(&clientmodels.Chat{
			ID: activityIds.ChatID,
			Members: []clientmodels.ChatMember{
				{
					UserID: "t" + user1.Id,
				},
				{
					UserID: "t" + senderUser.Id,
				},
				{
					UserID: "t" + user2.Id,
				},
			},
			Type: "G",
		}, nil).Times(1)
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(
			&clientmodels.Message{
				ID:              activityIds.MessageID,
				UserID:          "t" + senderUser.Id,
				ChatID:          activityIds.ChatID,
				UserDisplayName: senderUser.GetDisplayName(model.ShowFullName),
				Text:            "message",
				CreateAt:        now,
				LastUpdateAt:    now,
			}, nil).Times(1)

		discardReason := th.p.activityHandler.handleUpdatedActivity(msg, subscriptionID, activityIds)
		assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
	})

	t.Run("chats", func(t *testing.T) {
		type parameters struct {
			SyncChats         bool
			SyncNotifications bool
		}
		runPermutations(t, parameters{}, func(t *testing.T, params parameters) {
			th.Reset(t)

			th.setPluginConfigurationTemporarily(t, func(c *configuration) {
				c.SyncChats = params.SyncChats
				c.SyncNotifications = params.SyncNotifications
			})

			t.Run("chat message", func(t *testing.T) {
				th.Reset(t)

				senderUser := th.SetupUser(t, team)
				user1 := th.SetupUser(t, team)

				channel, err := th.p.apiClient.Channel.GetDirect(senderUser.Id, user1.Id)
				require.NoError(t, err)

				post := &model.Post{
					UserId:    senderUser.Id,
					ChannelId: channel.Id,
					Message:   "message",
				}
				err = th.p.apiClient.Post.CreatePost(post)
				require.NoError(t, err)

				// We connect after sending the post, since we're only simulating
				// the update for now.
				th.ConnectUser(t, user1.Id)
				th.ConnectUser(t, senderUser.Id)

				// Simulate the post having originated from Mattermost. Later, we'll let the code
				// do this itself once.
				messageID := model.NewId()
				chatID := model.NewId()
				postInfo := storemodels.PostInfo{
					MattermostID:        post.Id,
					MSTeamsID:           messageID,
					MSTeamsChannel:      chatID,
					MSTeamsLastUpdateAt: time.Now(),
				}
				err = th.p.GetStore().LinkPosts(postInfo)
				require.NoError(t, err)

				msg := (*clientmodels.Message)(nil)
				subscriptionID := "test"
				activityIds := clientmodels.ActivityIds{
					ChatID:    chatID,
					MessageID: messageID,
				}

				mockTeams := newMockTeamsHelper(th)
				mockTeams.registerChat(activityIds.ChatID, []*model.User{user1, senderUser})
				mockTeams.registerChatMessage(activityIds.ChatID, activityIds.MessageID, senderUser, "message updated")

				discardReason := th.p.activityHandler.handleUpdatedActivity(msg, subscriptionID, activityIds)
				if params.SyncChats && !params.SyncNotifications {
					assert.Equal(t, metrics.DiscardedReasonNone, discardReason)

					updatedPost, err := th.p.apiClient.Post.GetPost(post.Id)
					require.NoError(t, err)
					assert.Equal(t, "message updated", updatedPost.Message)
				} else {
					if params.SyncNotifications {
						assert.Equal(t, metrics.DiscardedReasonNotificationsOnly, discardReason)
					} else {
						assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
					}

					updatedPost, err := th.p.apiClient.Post.GetPost(post.Id)
					require.NoError(t, err)
					assert.Equal(t, "message", updatedPost.Message)
				}
			})

			t.Run("group chat message", func(t *testing.T) {
				th.Reset(t)

				senderUser := th.SetupUser(t, team)
				user1 := th.SetupUser(t, team)
				user2 := th.SetupUser(t, team)

				channel, err := th.p.apiClient.Channel.GetGroup([]string{senderUser.Id, user1.Id, user2.Id})
				require.NoError(t, err)

				post := &model.Post{
					UserId:    senderUser.Id,
					ChannelId: channel.Id,
					Message:   "message",
				}
				err = th.p.apiClient.Post.CreatePost(post)
				require.NoError(t, err)

				// We connect after sending the post, since we're only simulating
				// the update for now.
				th.ConnectUser(t, senderUser.Id)
				th.ConnectUser(t, user1.Id)
				th.ConnectUser(t, user2.Id)

				// Simulate the post having originated from Mattermost. Later, we'll let the code
				// do this itself once.
				messageID := model.NewId()
				chatID := model.NewId()
				postInfo := storemodels.PostInfo{
					MattermostID:        post.Id,
					MSTeamsID:           messageID,
					MSTeamsChannel:      chatID,
					MSTeamsLastUpdateAt: time.Now(),
				}
				err = th.p.GetStore().LinkPosts(postInfo)
				require.NoError(t, err)

				msg := (*clientmodels.Message)(nil)
				subscriptionID := "test"
				activityIds := clientmodels.ActivityIds{
					ChatID:    chatID,
					MessageID: messageID,
				}

				mockTeams := newMockTeamsHelper(th)
				mockTeams.registerGroupChat(activityIds.ChatID, []*model.User{user1, user2, senderUser})
				mockTeams.registerChatMessage(activityIds.ChatID, activityIds.MessageID, senderUser, "message updated")

				discardReason := th.p.activityHandler.handleUpdatedActivity(msg, subscriptionID, activityIds)
				if params.SyncChats && !params.SyncNotifications {
					assert.Equal(t, metrics.DiscardedReasonNone, discardReason)

					updatedPost, err := th.p.apiClient.Post.GetPost(post.Id)
					require.NoError(t, err)
					assert.Equal(t, "message updated", updatedPost.Message)
				} else {
					if params.SyncNotifications {
						assert.Equal(t, metrics.DiscardedReasonNotificationsOnly, discardReason)
					} else {
						assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
					}

					updatedPost, err := th.p.apiClient.Post.GetPost(post.Id)
					require.NoError(t, err)
					assert.Equal(t, "message", updatedPost.Message)
				}
			})
		})
	})

	t.Run("channels", func(t *testing.T) {
		type parameters struct {
			SyncLinkedChannels bool
			SyncNotifications  bool
		}
		runPermutations(t, parameters{}, func(t *testing.T, params parameters) {
			th.Reset(t)

			th.setPluginConfigurationTemporarily(t, func(c *configuration) {
				c.SyncLinkedChannels = params.SyncLinkedChannels
				c.SyncNotifications = params.SyncNotifications
			})

			t.Run("linked channel", func(t *testing.T) {
				th.Reset(t)

				senderUser := th.SetupUser(t, team)
				user1 := th.SetupUser(t, team)
				user2 := th.SetupUser(t, team)

				channel := th.SetupPublicChannel(t, team, WithMembers(user1))
				_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
				require.Nil(t, appErr)

				channelLink := storemodels.ChannelLink{
					MattermostTeamID:    team.Id,
					MattermostChannelID: channel.Id,
					MSTeamsTeam:         model.NewId(),
					MSTeamsChannel:      model.NewId(),
					Creator:             user1.Id,
				}

				err := th.p.store.StoreChannelLink(&channelLink)
				require.NoError(t, err)

				post := &model.Post{
					UserId:    senderUser.Id,
					ChannelId: channel.Id,
					Message:   "message",
				}
				err = th.p.apiClient.Post.CreatePost(post)
				require.NoError(t, err)

				messageID := model.NewId()
				postInfo := storemodels.PostInfo{
					MattermostID:        post.Id,
					MSTeamsID:           messageID,
					MSTeamsChannel:      channelLink.MSTeamsChannel,
					MSTeamsLastUpdateAt: time.Now(),
				}
				err = th.p.GetStore().LinkPosts(postInfo)
				require.NoError(t, err)

				th.ConnectUser(t, senderUser.Id)
				th.ConnectUser(t, user1.Id)
				th.ConnectUser(t, user2.Id)

				msg := (*clientmodels.Message)(nil)
				subscriptionID := "test"
				activityIds := clientmodels.ActivityIds{
					TeamID:    channelLink.MSTeamsTeam,
					ChannelID: channelLink.MSTeamsChannel,
					MessageID: messageID,
				}

				mockTeams := newMockTeamsHelper(th)
				mockTeams.registerMessage(activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID, senderUser, "message updated")

				discardReason := th.p.activityHandler.handleUpdatedActivity(msg, subscriptionID, activityIds)

				if params.SyncLinkedChannels {
					assert.Equal(t, metrics.DiscardedReasonNone, discardReason)

					updatedPost, err := th.p.apiClient.Post.GetPost(post.Id)
					require.NoError(t, err)
					assert.Equal(t, "message updated", updatedPost.Message)
				} else {
					assert.Equal(t, metrics.DiscardedReasonLinkedChannelsDisabled, discardReason)

					updatedPost, err := th.p.apiClient.Post.GetPost(post.Id)
					require.NoError(t, err)
					assert.Equal(t, "message", updatedPost.Message)
				}
			})
		})
	})
}

func TestHandleDeletedActivity(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("chats", func(t *testing.T) {
		t.Run("unknown post", func(t *testing.T) {
			th.Reset(t)

			th.setPluginConfigurationTemporarily(t, func(c *configuration) {
				c.SyncChats = true
			})

			messageID := model.NewId()
			chatID := model.NewId()

			activityIds := clientmodels.ActivityIds{
				ChatID:    chatID,
				MessageID: messageID,
			}

			discardReason := th.p.activityHandler.handleDeletedActivity(activityIds)
			assert.Equal(t, metrics.DiscardedReasonOther, discardReason)
		})

		type parameters struct {
			SyncChats         bool
			SyncNotifications bool
		}
		runPermutations(t, parameters{}, func(t *testing.T, params parameters) {
			th.Reset(t)

			th.setPluginConfigurationTemporarily(t, func(c *configuration) {
				c.SyncChats = params.SyncChats
				c.SyncNotifications = params.SyncNotifications
			})

			t.Run("chat message", func(t *testing.T) {
				th.Reset(t)

				senderUser := th.SetupUser(t, team)
				user1 := th.SetupUser(t, team)

				channel, err := th.p.apiClient.Channel.GetDirect(senderUser.Id, user1.Id)
				require.NoError(t, err)

				post := &model.Post{
					UserId:    senderUser.Id,
					ChannelId: channel.Id,
					Message:   "message",
				}
				err = th.p.apiClient.Post.CreatePost(post)
				require.NoError(t, err)

				// We connect after sending the post, since we're only simulating
				// the update for now.
				th.ConnectUser(t, user1.Id)
				th.ConnectUser(t, senderUser.Id)

				// Simulate the post having originated from Mattermost. Later, we'll let the code
				// do this itself once.
				messageID := model.NewId()
				chatID := model.NewId()
				postInfo := storemodels.PostInfo{
					MattermostID:        post.Id,
					MSTeamsID:           messageID,
					MSTeamsChannel:      chatID,
					MSTeamsLastUpdateAt: time.Now(),
				}
				err = th.p.GetStore().LinkPosts(postInfo)
				require.NoError(t, err)

				activityIds := clientmodels.ActivityIds{
					ChatID:    chatID,
					MessageID: messageID,
				}

				discardReason := th.p.activityHandler.handleDeletedActivity(activityIds)
				if params.SyncChats && !params.SyncNotifications {
					assert.Equal(t, metrics.DiscardedReasonNone, discardReason)

					_, err := th.p.apiClient.Post.GetPost(post.Id)
					require.ErrorContains(t, err, "not found")
				} else {
					if params.SyncNotifications {
						assert.Equal(t, metrics.DiscardedReasonNotificationsOnly, discardReason)
					} else {
						assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
					}

					unchangedPost, err := th.p.apiClient.Post.GetPost(post.Id)
					require.NoError(t, err)
					assert.Equal(t, post, unchangedPost)
				}
			})

			t.Run("group chat message", func(t *testing.T) {
				th.Reset(t)

				senderUser := th.SetupUser(t, team)
				user1 := th.SetupUser(t, team)
				user2 := th.SetupUser(t, team)

				channel, err := th.p.apiClient.Channel.GetGroup([]string{senderUser.Id, user1.Id, user2.Id})
				require.NoError(t, err)

				post := &model.Post{
					UserId:    senderUser.Id,
					ChannelId: channel.Id,
					Message:   "message",
				}
				err = th.p.apiClient.Post.CreatePost(post)
				require.NoError(t, err)

				// We connect after sending the post, since we're only simulating
				// the update for now.
				th.ConnectUser(t, senderUser.Id)
				th.ConnectUser(t, user1.Id)
				th.ConnectUser(t, user2.Id)

				// Simulate the post having originated from Mattermost. Later, we'll let the code
				// do this itself once.
				messageID := model.NewId()
				chatID := model.NewId()
				postInfo := storemodels.PostInfo{
					MattermostID:        post.Id,
					MSTeamsID:           messageID,
					MSTeamsChannel:      chatID,
					MSTeamsLastUpdateAt: time.Now(),
				}
				err = th.p.GetStore().LinkPosts(postInfo)
				require.NoError(t, err)

				activityIds := clientmodels.ActivityIds{
					ChatID:    chatID,
					MessageID: messageID,
				}

				mockTeams := newMockTeamsHelper(th)
				mockTeams.registerGroupChat(activityIds.ChatID, []*model.User{user1, user2, senderUser})
				mockTeams.registerChatMessage(activityIds.ChatID, activityIds.MessageID, senderUser, "message updated")

				discardReason := th.p.activityHandler.handleDeletedActivity(activityIds)
				if params.SyncChats && !params.SyncNotifications {
					assert.Equal(t, metrics.DiscardedReasonNone, discardReason)

					_, err := th.p.apiClient.Post.GetPost(post.Id)
					require.ErrorContains(t, err, "not found")
				} else {
					if params.SyncNotifications {
						assert.Equal(t, metrics.DiscardedReasonNotificationsOnly, discardReason)
					} else {
						assert.Equal(t, metrics.DiscardedReasonChatsDisabled, discardReason)
					}

					unchangedPost, err := th.p.apiClient.Post.GetPost(post.Id)
					require.NoError(t, err)
					assert.Equal(t, post, unchangedPost)
				}
			})
		})
	})

	t.Run("channels", func(t *testing.T) {
		t.Run("unknown post", func(t *testing.T) {
			th.Reset(t)

			th.setPluginConfigurationTemporarily(t, func(c *configuration) {
				c.SyncLinkedChannels = true
			})

			messageID := model.NewId()

			activityIds := clientmodels.ActivityIds{
				MessageID: messageID,
			}

			discardReason := th.p.activityHandler.handleDeletedActivity(activityIds)
			assert.Equal(t, metrics.DiscardedReasonOther, discardReason)
		})

		type parameters struct {
			SyncLinkedChannels bool
			SyncNotifications  bool
		}
		runPermutations(t, parameters{}, func(t *testing.T, params parameters) {
			th.Reset(t)

			th.setPluginConfigurationTemporarily(t, func(c *configuration) {
				c.SyncLinkedChannels = params.SyncLinkedChannels
				c.SyncNotifications = params.SyncNotifications
			})

			t.Run("linked channel", func(t *testing.T) {
				th.Reset(t)

				senderUser := th.SetupUser(t, team)
				user1 := th.SetupUser(t, team)
				user2 := th.SetupUser(t, team)

				channel := th.SetupPublicChannel(t, team, WithMembers(user1))
				_, appErr := th.p.API.UpdateChannelMemberRoles(channel.Id, user1.Id, model.ChannelAdminRoleId)
				require.Nil(t, appErr)

				channelLink := storemodels.ChannelLink{
					MattermostTeamID:    team.Id,
					MattermostChannelID: channel.Id,
					MSTeamsTeam:         model.NewId(),
					MSTeamsChannel:      model.NewId(),
					Creator:             user1.Id,
				}

				err := th.p.store.StoreChannelLink(&channelLink)
				require.NoError(t, err)

				post := &model.Post{
					UserId:    senderUser.Id,
					ChannelId: channel.Id,
					Message:   "message",
				}
				err = th.p.apiClient.Post.CreatePost(post)
				require.NoError(t, err)

				messageID := model.NewId()
				postInfo := storemodels.PostInfo{
					MattermostID:        post.Id,
					MSTeamsID:           messageID,
					MSTeamsChannel:      channelLink.MSTeamsChannel,
					MSTeamsLastUpdateAt: time.Now(),
				}
				err = th.p.GetStore().LinkPosts(postInfo)
				require.NoError(t, err)

				th.ConnectUser(t, senderUser.Id)
				th.ConnectUser(t, user1.Id)
				th.ConnectUser(t, user2.Id)

				activityIds := clientmodels.ActivityIds{
					TeamID:    channelLink.MSTeamsTeam,
					ChannelID: channelLink.MSTeamsChannel,
					MessageID: messageID,
				}

				mockTeams := newMockTeamsHelper(th)
				mockTeams.registerMessage(activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID, senderUser, "message updated")

				discardReason := th.p.activityHandler.handleDeletedActivity(activityIds)
				if params.SyncLinkedChannels {
					assert.Equal(t, metrics.DiscardedReasonNone, discardReason)

					_, err := th.p.apiClient.Post.GetPost(post.Id)
					require.ErrorContains(t, err, "not found")
				} else {
					assert.Equal(t, metrics.DiscardedReasonLinkedChannelsDisabled, discardReason)

					unchangedPost, err := th.p.apiClient.Post.GetPost(post.Id)
					require.NoError(t, err)
					assert.Equal(t, post, unchangedPost)
				}
			})
		})
	})
}

func TestHandleReactions(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	assertReactions := func(t *testing.T, expected []*model.Reaction, actual []*model.Reaction) {
		t.Helper()

		// Ignore timestamps
		for i := range expected {
			expected[i].CreateAt = 0
			expected[i].UpdateAt = 0
		}

		// Ignore timestamps
		for i := range actual {
			actual[i].CreateAt = 0
			actual[i].UpdateAt = 0
		}

		assert.ElementsMatch(t, expected, actual)
	}

	t.Run("no existing or new reactions", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))

		post := &model.Post{
			UserId:    user1.Id,
			ChannelId: channel.Id,
			Message:   "message",
		}
		err := th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		isDirectOrGroupMessage := false
		reactions := []clientmodels.Reaction{}

		th.p.activityHandler.handleReactions(post.Id, channel.Id, isDirectOrGroupMessage, reactions)

		actualReactions, err := th.p.apiClient.Post.GetReactions(post.Id)
		require.NoError(t, err)
		assert.Empty(t, actualReactions)
	})

	t.Run("no change from existing user", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))

		post := &model.Post{
			UserId:    user1.Id,
			ChannelId: channel.Id,
			Message:   "message",
		}
		err := th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		err = th.p.apiClient.Post.AddReaction(&model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
		})
		require.NoError(t, err)

		isDirectOrGroupMessage := false
		reactions := []clientmodels.Reaction{
			{
				UserID:   "t" + user1.Id,
				Reaction: "like",
			},
		}

		th.p.activityHandler.handleReactions(post.Id, channel.Id, isDirectOrGroupMessage, reactions)

		actualReactions, err := th.p.apiClient.Post.GetReactions(post.Id)
		require.NoError(t, err)

		expectedReactions := []*model.Reaction{
			{
				UserId:    user1.Id,
				PostId:    post.Id,
				ChannelId: channel.Id,
				EmojiName: "+1",
			},
		}
		assertReactions(t, expectedReactions, actualReactions)
	})

	t.Run("user removed reaction", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))

		post := &model.Post{
			UserId:    user1.Id,
			ChannelId: channel.Id,
			Message:   "message",
		}
		err := th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		err = th.p.apiClient.Post.AddReaction(&model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
		})
		require.NoError(t, err)

		isDirectOrGroupMessage := false
		reactions := []clientmodels.Reaction{
			{
				UserID:   "t" + model.NewId(),
				Reaction: "+1",
			},
		}

		th.p.activityHandler.handleReactions(post.Id, channel.Id, isDirectOrGroupMessage, reactions)

		actualReactions, err := th.p.apiClient.Post.GetReactions(post.Id)
		require.NoError(t, err)

		expectedReactions := []*model.Reaction{}
		assertReactions(t, expectedReactions, actualReactions)
	})

	t.Run("user changed reaction", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		channel := th.SetupPublicChannel(t, team, WithMembers(user1))

		post := &model.Post{
			UserId:    user1.Id,
			ChannelId: channel.Id,
			Message:   "message",
		}
		err := th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		err = th.p.apiClient.Post.AddReaction(&model.Reaction{
			UserId:    user1.Id,
			PostId:    post.Id,
			EmojiName: "+1",
		})
		require.NoError(t, err)

		isDirectOrGroupMessage := false
		reactions := []clientmodels.Reaction{
			{
				UserID:   "t" + user1.Id,
				Reaction: "sad",
			},
		}

		th.p.activityHandler.handleReactions(post.Id, channel.Id, isDirectOrGroupMessage, reactions)

		actualReactions, err := th.p.apiClient.Post.GetReactions(post.Id)
		require.NoError(t, err)

		expectedReactions := []*model.Reaction{
			{
				UserId:    user1.Id,
				PostId:    post.Id,
				ChannelId: channel.Id,
				EmojiName: "cry",
			},
		}
		assertReactions(t, expectedReactions, actualReactions)
	})
}
