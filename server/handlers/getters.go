package handlers

import (
	"encoding/base32"

	"github.com/gosimple/slug"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
)

func (ah *ActivityHandler) getMessageFromChat(chat *msteams.Chat, messageID string) (*msteams.Message, error) {
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
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to get original post", "error", err)
		return nil, err
	}
	return msg, nil
}

func (ah *ActivityHandler) getReplyFromChannel(userID string, teamID, channelID, messageID, replyID string) (*msteams.Message, error) {
	client, err := ah.plugin.GetClientForUser(userID)
	if err != nil {
		ah.plugin.GetAPI().LogError("unable to get bot client", "error", err)
		return nil, err
	}

	var msg *msteams.Message
	msg, err = client.GetReply(teamID, channelID, messageID, replyID)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to get original post", "error", err)
		return nil, err
	}
	return msg, nil
}

func (ah *ActivityHandler) getMessageFromChannel(userID string, teamID, channelID, messageID string) (*msteams.Message, error) {
	client, err := ah.plugin.GetClientForUser(userID)
	if err != nil {
		ah.plugin.GetAPI().LogError("unable to get bot client", "error", err)
		return nil, err
	}

	msg, err := client.GetMessage(teamID, channelID, messageID)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to get original post", "error", err)
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

func (ah *ActivityHandler) getMessageAndChatFromActivityIds(activityIds msteams.ActivityIds) (*msteams.Message, *msteams.Chat, error) {
	if activityIds.ChatID != "" {
		chat, err := ah.plugin.GetClientForApp().GetChat(activityIds.ChatID)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to get original chat", "error", err.Error())
			return nil, nil, err
		}
		msg, err := ah.getMessageFromChat(chat, activityIds.MessageID)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to get original message", "error", err.Error())
			return nil, nil, err
		}
		return msg, chat, nil
	}

	userID := ah.getUserIDForChannelLink(activityIds.TeamID, activityIds.ChannelID)

	if activityIds.ReplyID != "" {
		msg, err := ah.getReplyFromChannel(userID, activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID, activityIds.ReplyID)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to get original post", "error", err)
			return nil, nil, err
		}
		return msg, nil, nil
	}

	msg, err := ah.getMessageFromChannel(userID, activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to get original post", "error", err)
		return nil, nil, err
	}
	return msg, nil, nil
}

func (ah *ActivityHandler) getOrCreateSyntheticUser(userID, displayName string) (string, error) {
	mmUserID, err := ah.plugin.GetStore().TeamsToMattermostUserID(userID)
	if err != nil || mmUserID == "" {
		u, appErr := ah.plugin.GetAPI().GetUserByEmail(userID + "@msteamssync")
		if appErr != nil {
			var appErr2 *model.AppError
			memberUUID := uuid.Parse(userID)
			encoding := base32.NewEncoding("ybndrfg8ejkmcpqxot1uwisza345h769").WithPadding(base32.NoPadding)
			shortUserID := encoding.EncodeToString(memberUUID)
			u, appErr2 = ah.plugin.GetAPI().CreateUser(&model.User{
				Username:  slug.Make(displayName) + "-" + userID,
				FirstName: displayName,
				Email:     userID + "@msteamssync",
				Password:  model.NewId(),
				RemoteId:  &shortUserID,
			})
			if appErr2 != nil {
				return "", appErr2
			}
		}
		if err = ah.plugin.GetStore().SetUserInfo(u.Id, userID, nil); err != nil {
			ah.plugin.GetAPI().LogError("Unable to link the new created mirror user", "error", err.Error())
		}
		mmUserID = u.Id
	}
	return mmUserID, err
}

func (ah *ActivityHandler) getChatChannelID(chat *msteams.Chat, msteamsUserID string) (string, error) {
	userIDs := []string{}
	for _, member := range chat.Members {
		mmUserID, err := ah.getOrCreateSyntheticUser(member.UserID, member.DisplayName)
		if err != nil {
			return "", err
		}
		userIDs = append(userIDs, mmUserID)
	}
	if len(userIDs) < 2 {
		return "", errors.New("not enough user for creating a channel")
	}

	if chat.Type == "D" {
		channel, appErr := ah.plugin.GetAPI().GetDirectChannel(userIDs[0], userIDs[1])
		if appErr != nil {
			return "", appErr
		}
		return channel.Id, nil
	}
	if chat.Type == "G" {
		channel, appErr := ah.plugin.GetAPI().GetGroupChannel(userIDs)
		if appErr != nil {
			return "", appErr
		}
		return channel.Id, nil
	}
	return "", errors.New("dm/gm not found")
}
