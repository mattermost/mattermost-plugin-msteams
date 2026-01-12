// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"errors"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
)

func TestHandleCreatedActivity(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("ignore non-chats", func(t *testing.T) {
		th.Reset(t)

		activityIds := clientmodels.ActivityIds{
			TeamID:    model.NewId(),
			ChannelID: model.NewId(),
		}

		discardReason := th.p.activityHandler.handleCreatedActivity(activityIds)
		assert.Equal(t, metrics.DiscardedReasonChannelNotificationsUnsupported, discardReason)
	})

	t.Run("unable to get original get", func(t *testing.T) {
		th.Reset(t)

		activityIds := clientmodels.ActivityIds{
			ChatID: "invalid_chat_id",
		}

		th.appClientMock.On("GetChat", activityIds.ChatID).Return(nil, errors.New("Error while getting original chat")).Times(1)

		discardReason := th.p.activityHandler.handleCreatedActivity(activityIds)
		assert.Equal(t, metrics.DiscardedReasonUnableToGetTeamsData, discardReason)
	})

	t.Run("no connected users to get message", func(t *testing.T) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)
		user1 := th.SetupUser(t, team)

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

		discardReason := th.p.activityHandler.handleCreatedActivity(activityIds)
		assert.Equal(t, metrics.DiscardedReasonNoConnectedUser, discardReason)
	})

	t.Run("failed to get chat message", func(t *testing.T) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)
		th.ConnectUser(t, senderUser.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

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
		th.clientMock.On("GetChatMessage", activityIds.ChatID, activityIds.MessageID).Return(nil, errors.New("failed to get chat message")).Times(1)

		discardReason := th.p.activityHandler.handleCreatedActivity(activityIds)
		assert.Equal(t, metrics.DiscardedReasonUnableToGetTeamsData, discardReason)
	})

	t.Run("skipping not user event", func(t *testing.T) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)
		th.ConnectUser(t, senderUser.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

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

		discardReason := th.p.activityHandler.handleCreatedActivity(activityIds)
		assert.Equal(t, metrics.DiscardedReasonNotUserEvent, discardReason)
	})

	t.Run("notifications", func(t *testing.T) {
		type parameters struct {
			NotificationPref bool
			OnlineInTeams    bool
		}
		runPermutations(t, parameters{}, func(t *testing.T, params parameters) {
			th.Reset(t)

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

				activityIds := clientmodels.ActivityIds{
					ChatID:    "chat_id",
					MessageID: "message_id",
				}

				mockTeams := newMockTeamsHelper(th)
				mockTeams.registerChat(activityIds.ChatID, []*model.User{user1, senderUser})
				mockTeams.registerChatMessage(activityIds.ChatID, activityIds.MessageID, senderUser, "message")

				user1Presence := clientmodels.Presence{
					UserID: "t" + user1.Id,
				}
				if params.OnlineInTeams {
					user1Presence.Activity = PresenceActivityAvailable
					user1Presence.Availability = PresenceAvailabilityAvailable
				} else {
					user1Presence.Activity = PresenceActivityOffline
					user1Presence.Availability = PresenceAvailabilityOffline
				}

				th.appClientMock.On("GetPresencesForUsers", []string{"t" + user1.Id}).Return(map[string]clientmodels.Presence{
					"t" + user1.Id: user1Presence,
				}, nil).Times(1)

				discardReason := th.p.activityHandler.handleCreatedActivity(activityIds)
				assert.Equal(t, metrics.DiscardedReasonNone, discardReason)

				if params.NotificationPref && !params.OnlineInTeams {
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

				// user2 always prefers to get the notification
				err = th.p.setNotificationPreference(user2.Id, true)
				require.NoError(t, err)

				user3 := th.SetupUser(t, team)
				th.ConnectUser(t, user3.Id)

				// user3 always prefers to get the notification
				err = th.p.setNotificationPreference(user3.Id, true)
				require.NoError(t, err)

				botUser, err := th.p.apiClient.User.Get(th.p.botUserID)
				require.NoError(t, err)
				th.ConnectUser(t, botUser.Id)

				activityIds := clientmodels.ActivityIds{
					ChatID:    "chat_id",
					MessageID: "message_id",
				}

				mockTeams := newMockTeamsHelper(th)
				mockTeams.registerGroupChat(activityIds.ChatID, []*model.User{user1, user2, user3})
				mockTeams.registerChatMessage(activityIds.ChatID, activityIds.MessageID, senderUser, "message")

				user1Presence := clientmodels.Presence{
					UserID: "t" + user1.Id,
				}
				if params.OnlineInTeams {
					user1Presence.Activity = PresenceActivityAvailable
					user1Presence.Availability = PresenceAvailabilityAvailable
				} else {
					user1Presence.Activity = PresenceActivityOffline
					user1Presence.Availability = PresenceAvailabilityOffline
				}

				th.appClientMock.On("GetPresencesForUsers", []string{"t" + user1.Id, "t" + user2.Id, "t" + user3.Id}).Return(map[string]clientmodels.Presence{
					"t" + user1.Id: user1Presence,
					"t" + user2.Id: {
						UserID:       "t" + user2.Id,
						Activity:     PresenceActivityOffline,
						Availability: PresenceAvailabilityOffline,
					},
					// no presence for user3: should always get the message
				}, nil).Times(1)

				discardReason := th.p.activityHandler.handleCreatedActivity(activityIds)
				assert.Equal(t, metrics.DiscardedReasonNone, discardReason)

				if params.NotificationPref && !params.OnlineInTeams {
					th.assertDMFromUserRe(t, botUser.Id, user1.Id, "message")
				} else {
					th.assertNoDMFromUser(t, botUser.Id, user1.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
				}

				th.assertDMFromUserRe(t, botUser.Id, user2.Id, "message")
				th.assertDMFromUserRe(t, botUser.Id, user3.Id, "message")
			})
		})
	})
}
