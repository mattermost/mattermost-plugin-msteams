package main

import (
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
)

func (ah *ActivityHandler) getMessageFromChat(chat *clientmodels.Chat, messageID string) (*clientmodels.Message, error) {
	var client msteams.Client
	for _, member := range chat.Members {
		client, _ = ah.plugin.GetClientForTeamsUser(member.UserID)
		if client != nil {
			break
		}
	}
	if client == nil {
		return nil, nil
	}

	msg, err := client.GetChatMessage(chat.ID, messageID)
	if err != nil || msg == nil {
		ah.plugin.GetAPI().LogWarn("Unable to get message from chat", "chat_id", chat.ID, "message_id", messageID, "error", err)
		return nil, err
	}
	return msg, nil
}

func (ah *ActivityHandler) getMessageAndChatFromActivityIds(activityIds clientmodels.ActivityIds) (*clientmodels.Message, *clientmodels.Chat, error) {
	if activityIds.ChatID != "" {
		chat, err := ah.plugin.GetClientForApp().GetChat(activityIds.ChatID)
		if err != nil || chat == nil {
			ah.plugin.GetAPI().LogWarn("Unable to get original chat", "chat_id", activityIds.ChatID, "error", err)
			return nil, nil, err
		}
		msg, err := ah.getMessageFromChat(chat, activityIds.MessageID)
		if err != nil || msg == nil {
			return nil, nil, err
		}
		return msg, chat, nil
	}

	if activityIds.ReplyID != "" {
		msg, err := ah.plugin.GetClientForApp().GetReply(activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID, activityIds.ReplyID)
		if err != nil {
			ah.plugin.GetAPI().LogWarn(
				"Failed to get reply from channel",
				"teams_team_id", activityIds.TeamID,
				"teams_channel_id", activityIds.ChannelID,
				"teams_message_id", activityIds.MessageID,
				"teams_reply_id", activityIds.ReplyID,
				"error", err,
			)
			return nil, nil, err
		}

		return msg, nil, nil
	}

	msg, err := ah.plugin.GetClientForApp().GetMessage(activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID)
	if err != nil {
		ah.plugin.GetAPI().LogWarn(
			"Failed to get message from channel",
			"teams_team_id", activityIds.TeamID,
			"teams_channel_id", activityIds.ChannelID,
			"teams_message_id", activityIds.MessageID,
			"error", err,
		)

		return nil, nil, err
	}

	return msg, nil, nil
}
