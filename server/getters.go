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

func (ah *ActivityHandler) getReplyFromChannel(teamID, channelID, messageID, replyID string) (*clientmodels.Message, error) {
	msg, err := ah.plugin.GetClientForApp().GetReply(teamID, channelID, messageID, replyID)
	if err != nil {
		ah.plugin.GetAPI().LogWarn("Unable to get reply from channel", "reply_id", replyID, "error", err)
		return nil, err
	}
	return msg, nil
}

func (ah *ActivityHandler) getMessageFromChannel(teamID, channelID, messageID string) (*clientmodels.Message, error) {
	msg, err := ah.plugin.GetClientForApp().GetMessage(teamID, channelID, messageID)
	if err != nil {
		ah.plugin.GetAPI().LogWarn("Unable to get message from channel", "message_id", messageID, "error", err)
		return nil, err
	}
	return msg, nil
}

func (ah *ActivityHandler) getMessageAndChatFromActivityIds(providedMsg *clientmodels.Message, activityIds clientmodels.ActivityIds) (*clientmodels.Message, *clientmodels.Chat, error) {
	if activityIds.ChatID != "" {
		chat, err := ah.plugin.GetClientForApp().GetChat(activityIds.ChatID)
		if err != nil || chat == nil {
			ah.plugin.GetAPI().LogWarn("Unable to get original chat", "chat_id", activityIds.ChatID, "error", err)
			return nil, nil, err
		}
		if providedMsg != nil {
			return providedMsg, chat, nil
		}
		msg, err := ah.getMessageFromChat(chat, activityIds.MessageID)
		if err != nil || msg == nil {
			return nil, nil, err
		}
		return msg, chat, nil
	}

	if providedMsg != nil {
		return providedMsg, nil, nil
	}

	if activityIds.ReplyID != "" {
		msg, err := ah.getReplyFromChannel(activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID, activityIds.ReplyID)
		if err != nil {
			return nil, nil, err
		}

		return msg, nil, nil
	}

	msg, err := ah.getMessageFromChannel(activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID)
	if err != nil {
		return nil, nil, err
	}

	return msg, nil, nil
}
