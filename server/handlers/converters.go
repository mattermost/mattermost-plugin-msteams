package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattn/godown"
)

func (ah *ActivityHandler) GetAvatarURL(userID string) string {
	defaultAvatarURL := ah.plugin.GetURL() + "/public/msteams-sync-icon.svg"
	resp, err := http.Get(ah.plugin.GetURL() + "/avatar/" + userID)
	if err != nil {
		ah.plugin.GetAPI().LogDebug("Unable to get user avatar", "Error", err.Error())
		return defaultAvatarURL
	}

	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if strings.Contains(resp.Header.Get("Content-Type"), "image") {
		return ah.plugin.GetURL() + "/avatar/" + userID
	}

	return defaultAvatarURL
}

func (ah *ActivityHandler) msgToPost(userID, channelID string, msg *msteams.Message, senderID string) (*model.Post, error) {
	text := ah.handleMentions(msg)
	text = convertToMD(text)
	props := make(map[string]interface{})
	rootID := ""

	if msg.ReplyToID != "" {
		rootInfo, _ := ah.plugin.GetStore().GetPostInfoByMSTeamsID(msg.ChatID+msg.ChannelID, msg.ReplyToID)
		if rootInfo != nil {
			rootID = rootInfo.MattermostID
		}
	}

	newText, attachments, parentID := ah.handleAttachments(userID, channelID, text, msg)
	text = newText
	if parentID != "" {
		rootID = parentID
	}

	if rootID == "" && msg.Subject != "" {
		text = "## " + msg.Subject + "\n" + text
	}

	post := &model.Post{UserId: senderID, ChannelId: channelID, Message: text, Props: props, RootId: rootID, FileIds: attachments}
	post.AddProp("msteams_sync_"+ah.plugin.GetBotUserID(), true)

	if senderID == ah.plugin.GetBotUserID() {
		post.AddProp("override_username", msg.UserDisplayName)
		post.AddProp("from_webhook", "true")
		post.AddProp("override_icon_url", ah.GetAvatarURL(msg.UserID))
	}
	return post, nil
}

func (ah *ActivityHandler) handleMentions(msg *msteams.Message) string {
	for _, mention := range msg.Mentions {
		mmMention := ""
		if mention.UserID != "" {
			var mmUser *model.User
			var getErr *model.AppError
			mmUserID, err := ah.plugin.GetStore().TeamsToMattermostUserID(mention.UserID)
			if err != nil {
				msUser, mErr := ah.plugin.GetClientForApp().GetUser(mention.UserID)
				if mErr != nil {
					ah.plugin.GetAPI().LogDebug("Unable to get ms user", "Error", mErr.Error())
					continue
				}

				mmUser, getErr = ah.plugin.GetAPI().GetUserByEmail(msUser.Mail)
				if getErr != nil {
					ah.plugin.GetAPI().LogDebug("Unable to get mm user details", "Error", getErr.Error())
					continue
				}

				sErr := ah.plugin.GetStore().SetUserInfo(mmUser.Id, mention.UserID, nil)
				if sErr != nil {
					ah.plugin.GetAPI().LogDebug("Unable to store user info", "Error", sErr.Error())
				}
			} else {
				mmUser, getErr = ah.plugin.GetAPI().GetUser(mmUserID)
				if getErr != nil {
					ah.plugin.GetAPI().LogDebug("Unable to get mm user details", "Error", getErr.Error())
					continue
				}
			}

			mmMention = fmt.Sprintf("@%s ", mmUser.Username)
		} else {
			if mention.MentionedText == "Everyone" {
				mmMention = "@all"
			} else {
				mmMention = "@channel"
			}
		}

		msg.Text = strings.Replace(msg.Text, fmt.Sprintf("<at id=\"%s\">%s</at>", fmt.Sprint(mention.ID), mention.MentionedText), mmMention, 1)
	}

	return msg.Text
}

func convertToMD(text string) string {
	if !strings.Contains(text, "<div>") && !strings.Contains(text, "<p>") {
		return text
	}
	var sb strings.Builder
	err := godown.Convert(&sb, strings.NewReader(text), nil)
	if err != nil {
		return text
	}
	return sb.String()
}
