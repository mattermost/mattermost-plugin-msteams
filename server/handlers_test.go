package main

import (
	"errors"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
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

	t.Run("notifications", func(t *testing.T) {
		type parameters struct {
			NotificationPref bool
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
				assert.Equal(t, metrics.DiscardedReasonNone, discardReason)

				if params.NotificationPref {
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
				assert.Equal(t, metrics.DiscardedReasonNone, discardReason)

				if params.NotificationPref {
					th.assertDMFromUserRe(t, botUser.Id, user1.Id, "message")
				} else {
					th.assertNoDMFromUser(t, botUser.Id, user1.Id, model.GetMillisForTime(time.Now().Add(-5*time.Second)))
				}
			})
		})
	})
}
