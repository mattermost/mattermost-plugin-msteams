package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	metricsmocks "github.com/mattermost/mattermost-plugin-msteams/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest/mock"
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
				mockTeams.registerGroupChat(activityIds.ChatID, []*model.User{user1, user2, senderUser})
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
	msTeamsLastUpdateAtTime := time.Now()
	for _, testCase := range []struct {
		description  string
		activityIds  clientmodels.ActivityIds
		setupClient  func(*clientmocks.Client, *clientmocks.Client)
		setupAPI     func(*plugintest.API)
		setupStore   func(*storemocks.Store)
		setupMetrics func(*metricsmocks.Metrics)
	}{
		{
			description: "Unable to get original message",
			activityIds: clientmodels.ActivityIds{
				ChatID: "invalid-ChatID",
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", "invalid-ChatID").Return(nil, errors.New("error while getting original chat")).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore:   func(store *storemocks.Store) {},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {},
		},
		{
			description: "Message is nil",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Skipping not user event",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{}, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Skipping messages from bot user",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetTeamsUserID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Unable to get the post info",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(nil, nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Unable to get the post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetPost", "mockMattermostID").Return(nil, testutils.GetInternalServerAppError("unable to get the post")).Times(1)
			},
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID:        "mockMattermostID",
					MSTeamsID:           "mockMSTeamsID",
					MSTeamsChannel:      "mockMSTeamsChannel",
					MSTeamsLastUpdateAt: time.Now(),
				}, nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Unable to get and recover the post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				getPostError := testutils.GetInternalServerAppError("Unable to get the post.")
				getPostError.Id = "app.post.get.app_error"
				mockAPI.On("GetPost", "mockMattermostID").Return(nil, getPostError).Times(1)
			},
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID:        "mockMattermostID",
					MSTeamsID:           "mockMSTeamsID",
					MSTeamsChannel:      "mockMSTeamsChannel",
					MSTeamsLastUpdateAt: time.Now(),
				}, nil).Times(1)
				store.On("RecoverPost", "mockMattermostID").Return(errors.New("unable to recover"))
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Valid: chat message",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					ID: testutils.GetChatID(),
					Members: []clientmodels.ChatMember{
						{
							UserID: testutils.GetTeamsUserID(),
						},
					},
				}, nil).Times(1)
				uclient.On("GetChatMessage", testutils.GetChatID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					ChatID:          testutils.GetChatID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					LastUpdateAt:    msTeamsLastUpdateAtTime,
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetPost", "mockMattermostID").Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetSenderID(), time.Now().UnixMicro()), nil).Times(1)
				mockAPI.On("UpdatePost", mock.Anything).Return(nil, nil).Times(1)
				mockAPI.On("GetReactions", "mockMattermostID").Return([]*model.Reaction{}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetTeamsUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
				mockAPI.On("GetFileInfo", mock.Anything).Return(testutils.GetFileInfo(), nil).Once()
			},
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChatID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID:        "mockMattermostID",
					MSTeamsID:           "mockMSTeamsID",
					MSTeamsChannel:      "mockMSTeamsChannel",
					MSTeamsLastUpdateAt: time.Now(),
				}, nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetUserID()).Return(&fakeToken, nil)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMSTeams, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			description: "Valid: sync linked channels disabled",
			activityIds: clientmodels.ActivityIds{
				TeamID:    "mockTeamID",
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetMessage", "mockTeamID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					TeamID:          "mockTeamID",
					ChannelID:       testutils.GetChannelID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					LastUpdateAt:    msTeamsLastUpdateAtTime,
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetPost", "mockMattermostID").Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetSenderID(), time.Now().UnixMicro()), nil).Times(1)
				mockAPI.On("GetReactions", "mockMattermostID").Return([]*model.Reaction{}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
			},
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MSTeamsLastUpdateAt: time.Now(),
					MattermostID:        "mockMattermostID",
				}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
			},
		},
		{
			description: "Valid: channel message",
			activityIds: clientmodels.ActivityIds{
				TeamID:    "mockTeamID",
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				client.On("GetMessage", "mockTeamID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&clientmodels.Message{
					ID:              testutils.GetMessageID(),
					UserID:          testutils.GetSenderID(),
					TeamID:          "mockTeamID",
					ChannelID:       testutils.GetChannelID(),
					UserDisplayName: "mockUserDisplayName",
					Text:            "mockText",
					LastUpdateAt:    msTeamsLastUpdateAtTime,
				}, nil).Times(1)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetPost", "mockMattermostID").Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetSenderID(), time.Now().UnixMicro()), nil).Times(1)
				mockAPI.On("UpdatePost", mock.Anything).Return(nil, nil).Times(1)
				mockAPI.On("GetReactions", "mockMattermostID").Return([]*model.Reaction{}, nil).Times(1)
				mockAPI.On("GetUser", testutils.GetUserID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
			},
			setupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", "bot-user-id").Return(testutils.GetTeamsUserID(), nil).Times(1)
				store.On("TeamsToMattermostUserID", testutils.GetSenderID()).Return(testutils.GetUserID(), nil).Times(1)
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MSTeamsLastUpdateAt: time.Now(),
					MattermostID:        "mockMattermostID",
				}, nil).Times(1)
				store.On("GetLinkByMSTeamsChannelID", "mockTeamID", testutils.GetChannelID()).Return(&storemodels.ChannelLink{
					Creator:             "mockCreator",
					MattermostChannelID: testutils.GetChannelID(),
				}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMSTeams, false).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)

			testCase.setupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			testCase.setupStore(p.store.(*storemocks.Store))
			testCase.setupAPI(p.API.(*plugintest.API))
			testCase.setupMetrics(p.metricsService.(*metricsmocks.Metrics))
			testutils.MockLogs(p.API.(*plugintest.API))
			subscriptionID := "test"

			ah := ActivityHandler{}
			ah.plugin = p
			discardedReason := ah.handleUpdatedActivity(nil, subscriptionID, testCase.activityIds)

			lastSubscriptionActivity, ok := ah.lastUpdateAtMap.Load(subscriptionID)
			if discardedReason == "" {
				assert.True(t, ok)
				assert.Equal(t, lastSubscriptionActivity, msTeamsLastUpdateAtTime)
			} else {
				assert.False(t, ok)
			}
		})
	}
}

func TestHandleDeletedActivity(t *testing.T) {
	for _, testCase := range []struct {
		description  string
		activityIds  clientmodels.ActivityIds
		setupAPI     func(*plugintest.API)
		setupStore   func(*storemocks.Store)
		setupMetrics func(*metricsmocks.Metrics)
	}{
		{
			description: "Successfully deleted post",
			activityIds: clientmodels.ActivityIds{
				ChatID:    testutils.GetChatID(),
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("DeletePost", testutils.GetMattermostID()).Return(nil).Times(1)
			},
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", fmt.Sprintf("%s%s", testutils.GetChatID(), testutils.GetChannelID()), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetMattermostID(),
				}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionDeleted, metrics.ActionSourceMSTeams, true).Times(1)
			},
		},
		{
			description: "Unable to get post info by MS teams ID",
			activityIds: clientmodels.ActivityIds{
				ChannelID: testutils.GetChannelID(),
			},
			setupAPI: func(mockAPI *plugintest.API) {},
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), "").Return(nil, errors.New("Error while getting post info by MS teams ID")).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {},
		},
		{
			description: "Unable to to delete post",
			activityIds: clientmodels.ActivityIds{
				ChannelID: testutils.GetChannelID(),
				MessageID: testutils.GetMessageID(),
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("DeletePost", "").Return(&model.AppError{
					Message: "Error while deleting a post",
				}).Times(1)
			},
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{}, nil).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			testCase.setupStore(p.store.(*storemocks.Store))
			testCase.setupAPI(p.API.(*plugintest.API))
			testCase.setupMetrics(p.metricsService.(*metricsmocks.Metrics))
			testutils.MockLogs(p.API.(*plugintest.API))
			ah := ActivityHandler{}
			ah.plugin = p

			ah.handleDeletedActivity(testCase.activityIds)
		})
	}
}

func TestHandleReactions(t *testing.T) {
	for _, testCase := range []struct {
		description  string
		reactions    []clientmodels.Reaction
		setupAPI     func(*plugintest.API)
		setupStore   func(*storemocks.Store)
		setupMetrics func(*metricsmocks.Metrics)
	}{
		{
			description: "Reactions list is empty",
			reactions:   []clientmodels.Reaction{},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetReactions", testutils.GetPostID()).Return([]*model.Reaction{}, nil).Times(1)
			},
			setupStore:   func(store *storemocks.Store) {},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {},
		},
		{
			description: "Unable to get the reactions",
			reactions: []clientmodels.Reaction{
				{
					UserID:   testutils.GetTeamsUserID(),
					Reaction: "+1",
				},
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetReactions", testutils.GetPostID()).Return(nil, testutils.GetInternalServerAppError("unable to get the reaction")).Times(1)
			},
			setupStore:   func(store *storemocks.Store) {},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {},
		},
		{
			description: "Unable to find the user for the reaction",
			reactions: []clientmodels.Reaction{
				{
					UserID:   testutils.GetTeamsUserID(),
					Reaction: "+1",
				},
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetReactions", testutils.GetPostID()).Return([]*model.Reaction{
					{
						UserId:    testutils.GetTeamsUserID(),
						EmojiName: "+1",
						PostId:    testutils.GetPostID(),
					},
				}, nil).Times(1)

				mockAPI.On("RemoveReaction", &model.Reaction{
					UserId:    testutils.GetTeamsUserID(),
					EmojiName: "+1",
					PostId:    testutils.GetPostID(),
					ChannelId: "removedfromplugin",
				}).Return(nil).Times(1)
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return("", errors.New("unable to find the user for the reaction")).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMSTeams, false).Times(1)
			},
		},
		{
			description: "Unable to remove the reaction",
			reactions: []clientmodels.Reaction{
				{
					UserID:   testutils.GetTeamsUserID(),
					Reaction: "+1",
				},
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetReactions", testutils.GetPostID()).Return([]*model.Reaction{
					{
						UserId:    testutils.GetTeamsUserID(),
						EmojiName: "+1",
						PostId:    testutils.GetPostID(),
					},
				}, nil).Times(1)

				mockAPI.On("RemoveReaction", &model.Reaction{
					UserId:    testutils.GetTeamsUserID(),
					EmojiName: "+1",
					PostId:    testutils.GetPostID(),
					ChannelId: "removedfromplugin",
				}).Return(testutils.GetInternalServerAppError("unable to remove reaction")).Times(1)

				mockAPI.On("GetUser", testutils.GetID()).Return(testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), nil).Once()
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetID(), nil).Times(1)
			},
			setupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMSTeams, false).Times(1)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			testCase.setupStore(p.store.(*storemocks.Store))
			testCase.setupAPI(p.API.(*plugintest.API))
			testCase.setupMetrics(p.metricsService.(*metricsmocks.Metrics))
			testutils.MockLogs(p.API.(*plugintest.API))
			ah := ActivityHandler{}
			ah.plugin = p

			ah.handleReactions(testutils.GetPostID(), testutils.GetChannelID(), false, testCase.reactions)
		})
	}
}
