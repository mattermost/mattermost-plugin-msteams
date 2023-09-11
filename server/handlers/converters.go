package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/markdown"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-server/v6/model"
	"golang.org/x/net/html"
)

const hostedContentsStr = "hostedContents"

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

func (ah *ActivityHandler) msgToPost(channelID, senderID string, msg *msteams.Message, chat *msteams.Chat) (*model.Post, bool) {
	text := ah.handleMentions(msg)
	text = ah.handleEmojis(text)
	var embeddedImages []msteams.Attachment
	text, embeddedImages = ah.handleImages(text)
	msg.Attachments = append(msg.Attachments, embeddedImages...)
	text = markdown.ConvertToMD(text)
	props := make(map[string]interface{})
	rootID := ""

	if msg.ReplyToID != "" {
		rootInfo, _ := ah.plugin.GetStore().GetPostInfoByMSTeamsID(msg.ChatID+msg.ChannelID, msg.ReplyToID)
		if rootInfo != nil {
			rootID = rootInfo.MattermostID
		}
	}

	newText, attachments, parentID, errorFound := ah.handleAttachments(channelID, text, msg, chat)
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
	return post, errorFound
}

func (ah *ActivityHandler) handleMentions(msg *msteams.Message) string {
	userIDVsNames := make(map[string]string)
	if msg.ChatID != "" {
		for _, mention := range msg.Mentions {
			if userIDVsNames[mention.UserID] == "" {
				userIDVsNames[mention.UserID] = mention.MentionedText
			} else if userIDVsNames[mention.UserID] != mention.MentionedText {
				userIDVsNames[mention.UserID] += " " + mention.MentionedText
			}
		}
	}

	idx := 0
	for idx < len(msg.Mentions) {
		mmMention := ""
		mention := msg.Mentions[idx]
		idx++
		switch {
		case mention.UserID != "":
			mmUserID, err := ah.plugin.GetStore().TeamsToMattermostUserID(mention.UserID)
			if err != nil {
				ah.plugin.GetAPI().LogDebug("Unable to get MM user ID from Teams user ID", "TeamsUserID", mention.UserID, "Error", err.Error())
				continue
			}

			mmUser, getErr := ah.plugin.GetAPI().GetUser(mmUserID)
			if getErr != nil {
				ah.plugin.GetAPI().LogDebug("Unable to get MM user details", "MMUserID", mmUserID, "Error", getErr.DetailedError)
				continue
			}

			mmMention = fmt.Sprintf("@%s ", mmUser.Username)
		case mention.MentionedText == "Everyone" && mention.ConversationID == msg.ChatID:
			mmMention = "@all"
		case mention.ConversationID == msg.ChannelID:
			mmMention = "@channel"
		}

		if mmMention == "" {
			msg.Text = strings.Replace(msg.Text, fmt.Sprintf("<at id=\"%s\">%s</at>", fmt.Sprint(mention.ID), mention.MentionedText), mention.MentionedText, 1)
		} else {
			msg.Text = strings.Replace(msg.Text, fmt.Sprintf("<at id=\"%s\">%s</at>", fmt.Sprint(mention.ID), mention.MentionedText), mmMention, 1)
		}

		if idx < len(msg.Mentions) && len(strings.Fields(userIDVsNames[mention.UserID])) >= 2 {
			mention = msg.Mentions[idx]
			msg.Text = strings.Replace(msg.Text, fmt.Sprintf("&nbsp;<at id=\"%s\">%s</at>", fmt.Sprint(mention.ID), mention.MentionedText), "", 1)
			idx++
		}
	}

	return msg.Text
}

func (ah *ActivityHandler) handleEmojis(text string) string {
	emojisData := strings.Split(text, "</emoji>")

	for idx, emojiData := range emojisData {
		if idx == len(emojisData)-1 {
			break
		}

		emojiIdx := strings.Index(emojiData, "<emoji")
		if emojiIdx != -1 {
			emojiData = emojiData[emojiIdx:] + "</emoji>"
			doc, err := html.Parse(strings.NewReader(emojiData))
			if err != nil {
				ah.plugin.GetAPI().LogWarn("Unable to parse emoji data", "EmojiData", emojiData, "Error", err.Error())
				continue
			}

			for _, a := range doc.FirstChild.FirstChild.NextSibling.FirstChild.Attr {
				if a.Key == "alt" {
					text = strings.Replace(text, emojiData, a.Val, 1)
					break
				}
			}
		}
	}

	return text
}

func (ah *ActivityHandler) handleImages(text string) (string, []msteams.Attachment) {
	imageURLs := getImageTagsFromHTML(text)
	var attachments []msteams.Attachment
	for _, imageURL := range imageURLs {
		attachments = append(attachments, msteams.Attachment{
			ContentURL: imageURL,
		})
	}

	text = imageRE.ReplaceAllStringFunc(text, func(s string) string {
		if strings.Contains(s, hostedContentsStr) {
			return ""
		}

		return s
	})

	return text, attachments
}

func getImageTagsFromHTML(text string) []string {
	tokenizer := html.NewTokenizer(strings.NewReader(text))
	var images []string
	for {
		token := tokenizer.Next()
		switch {
		case token == html.ErrorToken:
			return images
		case token == html.StartTagToken:
			if t := tokenizer.Token(); t.Data == "img" {
				for _, a := range t.Attr {
					if a.Key == "src" && strings.Contains(a.Val, hostedContentsStr) {
						images = append(images, a.Val)
						break
					}
				}
			}
		}
	}
}
