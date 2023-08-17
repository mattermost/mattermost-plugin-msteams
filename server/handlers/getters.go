package handlers

import (
	"encoding/base32"
	"fmt"

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
	if err != nil || msg == nil {
		ah.plugin.GetAPI().LogError("Unable to get message from chat", "chatID", chat.ID, "messageID", messageID, "error", err)
		return nil, err
	}
	return msg, nil
}

func (ah *ActivityHandler) getReplyFromChannel(teamID, channelID, messageID, replyID string) (*msteams.Message, error) {
	msg, err := ah.plugin.GetClientForApp().GetReply(teamID, channelID, messageID, replyID)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to get reply from channel", "replyID", replyID, "error", err)
		return nil, err
	}
	return msg, nil
}

func (ah *ActivityHandler) getMessageFromChannel(teamID, channelID, messageID string) (*msteams.Message, error) {
	msg, err := ah.plugin.GetClientForApp().GetMessage(teamID, channelID, messageID)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to get message from channel", "messageID", messageID, "error", err)
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
		if err != nil || chat == nil {
			ah.plugin.GetAPI().LogError("Unable to get original chat", "chatID", activityIds.ChatID, "error", err)
			return nil, nil, err
		}
		msg, err := ah.getMessageFromChat(chat, activityIds.MessageID)
		if err != nil || msg == nil {
			return nil, nil, err
		}
		return msg, chat, nil
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

func (ah *ActivityHandler) getOrCreateSyntheticUser(user *msteams.User, createSyntheticUser bool) (string, error) {
	mmUserID, err := ah.plugin.GetStore().TeamsToMattermostUserID(user.ID)
	if err == nil && mmUserID != "" {
		return mmUserID, err
	}

	u, appErr := ah.plugin.GetAPI().GetUserByEmail(user.Mail)
	if appErr != nil {
		if !createSyntheticUser {
			return "", appErr
		}

		userDisplayName := user.DisplayName
		memberUUID := uuid.Parse(user.ID)
		encoding := base32.NewEncoding("ybndrfg8ejkmcpqxot1uwisza345h769").WithPadding(base32.NoPadding)
		shortUserID := encoding.EncodeToString(memberUUID)
		username := "msteams_" + slug.Make(userDisplayName)

		newMMUser := &model.User{
			Username:  username,
			FirstName: userDisplayName,
			Email:     user.Mail,
			Password:  ah.plugin.GenerateRandomPassword(),
			RemoteId:  &shortUserID,
		}
		newMMUser.SetDefaultNotifications()
		newMMUser.NotifyProps[model.EmailNotifyProp] = "false"

		userSuffixID := 1
		for {
			u, appErr = ah.plugin.GetAPI().CreateUser(newMMUser)

			if appErr != nil {
				if appErr.Id == "app.user.save.username_exists.app_error" {
					newMMUser.Username += "-" + fmt.Sprint(userSuffixID)
					userSuffixID++
					continue
				}

				return "", appErr
			}

			break
		}

		preferences := model.Preferences{model.Preference{
			UserId:   u.Id,
			Category: model.PreferenceCategoryNotifications,
			Name:     model.PreferenceNameEmailInterval,
			Value:    "0",
		}}
		if prefErr := ah.plugin.GetAPI().UpdatePreferencesForUser(u.Id, preferences); prefErr != nil {
			ah.plugin.GetAPI().LogError("Unable to disable email notifications for new user", "mmuserID", u.Id, "error", prefErr.Error())
		}
	}

	if err = ah.plugin.GetStore().SetUserInfo(u.Id, user.ID, nil); err != nil {
		ah.plugin.GetAPI().LogError("Unable to link the new created mirror user", "error", err.Error())
	}

	return u.Id, err
}

func (ah *ActivityHandler) getChatChannelID(chat *msteams.Chat) (string, error) {
	userIDs := []string{}
	for _, member := range chat.Members {
		msteamsUser, clientErr := ah.plugin.GetClientForApp().GetUser(member.UserID)
		if clientErr != nil {
			ah.plugin.GetAPI().LogError("Unable to get the MS Teams user", "error", clientErr.Error())
			continue
		}

		if msteamsUser.Type == msteamsUserTypeGuest && !ah.plugin.GetSyncGuestUsers() {
			if mmUserID, _ := ah.getOrCreateSyntheticUser(msteamsUser, false); mmUserID != "" && ah.isRemoteUser(mmUserID) {
				if appErr := ah.plugin.GetAPI().UpdateUserActive(mmUserID, false); appErr != nil {
					ah.plugin.GetAPI().LogDebug("Unable to deactivate user", "MMUserID", mmUserID, "Error", appErr.Error())
				}
			}

			ah.plugin.GetAPI().LogDebug("Skipping guest user while creating DM/GM", "Email", msteamsUser.Mail)
			continue
		}

		mmUserID, err := ah.getOrCreateSyntheticUser(msteamsUser, true)
		if err != nil {
			return "", err
		}
		userIDs = append(userIDs, mmUserID)
	}
	if len(userIDs) < 2 {
		return "", errors.New("not enough users for creating a channel")
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
