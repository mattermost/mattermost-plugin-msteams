package handlers

import (
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattn/godown"
)

func (ah *ActivityHandler) MsgToPost(userID, channelID string, msg *msteams.Message, senderID string) (*model.Post, error) {
	text := ConvertToMD(msg.Text)
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
		post.AddProp("override_icon_url", ah.plugin.GetURL()+"/avatar/"+msg.UserID)
		post.AddProp("from_webhook", "true")
	}
	return post, nil
}

func ConvertToMD(text string) string {
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
