package main

import (
	"database/sql"
	"fmt"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
)

func (ah *ActivityHandler) handleCreatedActivityNotification(msg *clientmodels.Message, chat *clientmodels.Chat) string {
	if chat == nil {
		// We're only going to support notifications from chats for now.
		return metrics.DiscardedReasonChannelNotificationsUnsupported
	}

	// Get the presence status for each chat member to decide if we should relay into Mattermost.
	userIDs := make([]string, 0, len(chat.Members))
	for _, member := range chat.Members {
		// We won't notify senders about their own posts, so we don't need their presence.
		if member.UserID == msg.UserID {
			continue
		}

		userIDs = append(userIDs, member.UserID)
	}

	presences, err := ah.plugin.GetClientForApp().GetPresencesForUsers(userIDs)
	if err != nil {
		ah.plugin.GetAPI().LogWarn("Failed to fetch presence information for chat members", "chat_id", chat.ID, "message_id", msg.ID, "error", err)
	}

	botUserID := ah.plugin.GetBotUserID()

	chatLink := fmt.Sprintf("https://teams.microsoft.com/l/message/%s/%s?tenantId=%s&context={\"contextType\":\"chat\"}", chat.ID, msg.ID, ah.plugin.GetTenantID())
	isGroupChat := len(chat.Members) >= 3
	hasFilesUnknown := false
	for _, member := range chat.Members {
		// Don't notify senders about their own posts.
		if member.UserID == msg.UserID {
			continue
		}

		mattermostUserID, err := ah.plugin.GetStore().TeamsToMattermostUserID(member.UserID)
		if err == sql.ErrNoRows {
			continue
		} else if err != nil {
			ah.plugin.GetAPI().LogWarn("Failed to map Teams user to Mattermost user", "teams_user_id", member.UserID, "error", err)
			ah.plugin.metricsService.ObserveNotification(isGroupChat, hasFilesUnknown, metrics.DiscardedReasonInternalError)
			continue
		}

		if !ah.plugin.getNotificationPreference(mattermostUserID) {
			ah.plugin.GetAPI().LogInfo(
				"Skipping notification for chat member who disabled notifications",
				"user_id", mattermostUserID,
				"teams_user_id", member.UserID,
				"chat_id", chat.ID,
				"message_id", msg.ID,
			)
			ah.plugin.metricsService.ObserveNotification(isGroupChat, hasFilesUnknown, metrics.DiscardedReasonUserDisabledNotifications)
			continue
		}

		// Don't notify users active in Teams.
		if userPresenceIsActive(presences[member.UserID]) {
			ah.plugin.GetAPI().LogInfo(
				"Skipping notification for chat member present in Teams",
				"user_id", mattermostUserID,
				"teams_user_id", member.UserID,
				"chat_id", chat.ID,
				"message_id", msg.ID,
				"activity", presences[member.UserID].Activity,
				"availability", presences[member.UserID].Availability,
			)
			ah.plugin.metricsService.ObserveNotification(isGroupChat, hasFilesUnknown, metrics.DiscardedReasonUserActiveInTeams)
			continue
		}

		channel, err := ah.plugin.apiClient.Channel.GetDirect(mattermostUserID, ah.plugin.botUserID)
		if err != nil {
			ah.plugin.GetAPI().LogWarn("Failed to get bot DM channel with user", "user_id", mattermostUserID, "teams_user_id", member.UserID, "error", err)
			ah.plugin.metricsService.ObserveNotification(isGroupChat, hasFilesUnknown, metrics.DiscardedReasonInternalError)
			continue
		}

		post, skippedFileAttachments, _ := ah.msgToPost(channel.Id, botUserID, msg, chat, []string{})

		hasFiles := len(post.FileIds) > 0

		ah.plugin.GetAPI().LogInfo(
			"Delivered notification for chat member away from Teams",
			"user_id", mattermostUserID,
			"teams_user_id", member.UserID,
			"chat_id", chat.ID,
			"message_id", msg.ID,
			"activity", presences[member.UserID].Activity,
			"availability", presences[member.UserID].Availability,
		)
		ah.plugin.metricsService.ObserveNotification(isGroupChat, hasFiles, metrics.DiscardedReasonNone)
		ah.plugin.notifyChat(
			mattermostUserID,
			msg.UserDisplayName,
			chat.Topic,
			len(chat.Members),
			chatLink,
			post.Message,
			post.FileIds,
			skippedFileAttachments,
		)

		err = ah.plugin.GetStore().SetUserLastChatReceivedAt(mattermostUserID, storemodels.MilliToMicroSeconds(post.CreateAt))
		if err != nil {
			ah.plugin.GetAPI().LogWarn("Unable to set the last chat received at", "error", err)
		}
	}

	return metrics.DiscardedReasonNone
}

// Intentionally keep this block of code around as illustrative of what might be necessary to
// process channel notifications.
// // TODO: permissions
// for _, mention := range msg.Mentions {
// 	// Don't notify senders if they mention themselves.
// 	if mention.UserID == msg.UserID {
// 		continue
// 	}

// 	mattermostUserID, err := ah.plugin.GetStore().TeamsToMattermostUserID(mention.UserID)
// 	if err == sql.ErrNoRows {
// 		continue
// 	} else if err != nil {
// 		ah.plugin.GetAPI().LogWarn("Unable to map Teams user to Mattermost user", "teams_user_id", mention.UserID, "error", err)
// 		continue
// 	}

// 	botDMChannel, appErr := ah.plugin.GetAPI().GetDirectChannel(mattermostUserID, botUserID)
// 	if appErr != nil {
// 		ah.plugin.GetAPI().LogWarn("Unable to get direct channel with bot to send notification to user", "user_id", mattermostUserID, "error", appErr.Error())
// 		continue
// 	}

// 	notificationPost := post.Clone()
// 	notificationPost.ChannelId = botDMChannel.Id

// 	channelLink := fmt.Sprintf("https://teams.microsoft.com/l/message/%s/%s?tenantId=%s&parentMessageId=%s", msg.ChannelID, msg.ID, ah.plugin.GetTenantID(), msg.ID)
// 	notificationPost.Message = fmt.Sprintf("%s mentioned you in an MS Teams [channel](%s):\n> %s", msg.UserDisplayName, channelLink, notificationPost.Message)

// 	_, appErr = ah.plugin.GetAPI().CreatePost(notificationPost)
// 	if appErr != nil {
// 		ah.plugin.GetAPI().LogWarn("Unable to create notification post", "error", appErr)
// 	}
// }
// }
