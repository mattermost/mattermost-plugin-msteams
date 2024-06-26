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

	botUserID := ah.plugin.GetBotUserID()

	chatLink := fmt.Sprintf("https://teams.microsoft.com/l/message/%s/%s?tenantId=%s&context={\"contextType\":\"chat\"}", chat.ID, msg.ID, ah.plugin.GetTenantID())
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
			continue
		}

		if !ah.plugin.getNotificationPreference(mattermostUserID) {
			continue
		}

		channel, err := ah.plugin.apiClient.Channel.GetDirect(mattermostUserID, ah.plugin.botUserID)
		if err != nil {
			ah.plugin.GetAPI().LogWarn("Failed to get bot DM channel with user", "user_id", mattermostUserID, "teams_user_id", member.UserID, "error", err)
			continue
		}

		post, skippedFileAttachments, _ := ah.msgToPost(channel.Id, botUserID, msg, chat, []string{})

		ah.plugin.metricsService.ObserveNotification(len(chat.Members) >= 3, len(post.FileIds) > 0)
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
