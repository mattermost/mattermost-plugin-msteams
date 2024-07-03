package main

import (
	"database/sql"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
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

func (ah *ActivityHandler) getUserIDForChannelLink(teamID string, channelID string) string {
	channelLink, _ := ah.plugin.GetStore().GetLinkByMSTeamsChannelID(teamID, channelID)
	if channelLink != nil {
		return channelLink.Creator
	}
	return ah.plugin.GetBotUserID()
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

func (ah *ActivityHandler) getUser(user *clientmodels.User) (string, error) {
	// First see if we have an existing link recorded.
	mmUserID, err := ah.plugin.GetStore().TeamsToMattermostUserID(user.ID)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	} else if mmUserID != "" {
		return mmUserID, nil
	}

	// If none found, try to preemptively resolve the link by email.
	u, appErr := ah.plugin.GetAPI().GetUserByEmail(user.Mail)
	if appErr != nil {
		return "", appErr
	}

	// Ensure we save the link before we return it.
	if err = ah.plugin.GetStore().SetUserInfo(u.Id, user.ID, nil); err != nil {
		ah.plugin.GetAPI().LogWarn("Failed to link users after finding email match", "user_id", u.Id, "teams_user_id", user.ID, "error", err.Error())
		return "", err
	}

	return u.Id, nil
}

func (ah *ActivityHandler) getChatChannelAndUsersID(chat *clientmodels.Chat) (*model.Channel, []string, error) {
	userIDs := []string{}
	for _, member := range chat.Members {
		msteamsUser, clientErr := ah.plugin.GetClientForApp().GetUser(member.UserID)
		if clientErr != nil {
			ah.plugin.GetAPI().LogWarn("Unable to get the MS Teams user", "teams_user_id", member.UserID, "error", clientErr.Error())
			continue
		}

		if msteamsUser.Type == msteamsUserTypeGuest {
			continue
		}

		mmUserID, err := ah.getUser(msteamsUser)
		if err != nil {
			return nil, nil, err
		}
		userIDs = append(userIDs, mmUserID)
	}

	if len(userIDs) < 2 {
		return nil, nil, errors.New("not enough users for creating a channel")
	}

	if chat.Type == "D" {
		channel, appErr := ah.plugin.GetAPI().GetDirectChannel(userIDs[0], userIDs[1])
		if appErr != nil {
			return nil, nil, appErr
		}
		return channel, userIDs, nil
	}
	if chat.Type == "G" {
		channel, appErr := ah.plugin.GetAPI().GetGroupChannel(userIDs)
		if appErr != nil {
			return nil, nil, appErr
		}
		return channel, userIDs, nil
	}

	return nil, nil, errors.New("dm/gm not found")
}
