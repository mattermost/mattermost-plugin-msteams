// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"golang.org/x/net/html"

	"github.com/mattermost/mattermost-plugin-msteams/server/markdown"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
)

const hostedContentsStr = "hostedContents"

func (ah *ActivityHandler) msgToPost(channelID, senderID string, msg *clientmodels.Message, chat *clientmodels.Chat, existingFileIDs []string) (*model.Post, int, bool) {
	text := ah.handleMentions(msg)
	text = ah.handleEmojis(text)
	var embeddedImages []clientmodels.Attachment
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

	newText, attachments, parentID, skippedFileAttachments, errorFound := ah.handleAttachments(channelID, senderID, text, msg, chat, existingFileIDs)
	text = newText

	if parentID != "" {
		rootID = parentID
	}

	if rootID == "" && msg.Subject != "" {
		text = "## " + msg.Subject + "\n" + text
	}

	post := &model.Post{UserId: senderID, ChannelId: channelID, Message: text, Props: props, RootId: rootID, CreateAt: msg.CreateAt.UnixNano() / int64(time.Millisecond)}
	post.FileIds = attachments
	post.AddProp("msteams_sync_"+ah.plugin.GetBotUserID(), true)

	if senderID == ah.plugin.GetBotUserID() {
		post.AddProp("from_webhook", "true")
	}
	return post, skippedFileAttachments, errorFound
}

func (ah *ActivityHandler) handleMentions(msg *clientmodels.Message) string {
	// Teams sometimes translates an at-mention for a user like `Miguel De La Cruz` into four
	// discrete mentions. This seems broken, but at least easy to distinguish from genuinely
	// adjacent notifications as a result of injected &nbsp; between each mention. Find
	// anything that looks like that and collapse it back into a single mention.
	for i := 0; i < len(msg.Mentions); i++ {
		mention := msg.Mentions[i]

		// Scan for subsequent mentions, collapsing a pair at a time.
		for j := i + 1; j < len(msg.Mentions); j++ {
			nextMention := msg.Mentions[j]
			if nextMention.UserID != mention.UserID {
				break
			}

			multiWordMention := fmt.Sprintf(`<at id="%d">%s</at>`, mention.ID, mention.MentionedText)
			multiWordMention += fmt.Sprintf(`&nbsp;<at id="%d">%s</at>`, nextMention.ID, nextMention.MentionedText)

			// If the current mention + the next mention isn't actually found in the
			// prescribed format, then we might have genuinely adjacnent mentions for
			// the same user, so start over from that point.
			if !strings.Contains(msg.Text, multiWordMention) {
				i = j - 1
				break
			}

			// Replace the mention in place
			mention.MentionedText += fmt.Sprintf(" %s", nextMention.MentionedText)
			msg.Mentions[i] = mention

			// Then replace the text with the multi-word mentions back to just a single
			// mention.
			msg.Text = strings.Replace(msg.Text, multiWordMention, fmt.Sprintf(`<at id="%d">%s</at>`, mention.ID, mention.MentionedText), 1)
		}
	}

	for _, mention := range msg.Mentions {
		mmMention := ""
		switch {
		case mention.UserID != "":
			mmUserID, err := ah.plugin.GetStore().TeamsToMattermostUserID(mention.UserID)
			if err != nil {
				ah.plugin.GetAPI().LogWarn("Unable to get MM user ID from Teams user ID", "teams_user_id", mention.UserID, "error", err.Error())
				continue
			}

			mmUser, getErr := ah.plugin.GetAPI().GetUser(mmUserID)
			if getErr != nil {
				ah.plugin.GetAPI().LogWarn("Unable to get MM user details", "user_id", mmUserID, "error", getErr.DetailedError)
				continue
			}

			mmMention = fmt.Sprintf("@%s", mmUser.Username)
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
				ah.plugin.GetAPI().LogWarn("Unable to parse emoji data", "emoji_data", emojiData, "error", err.Error())
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

func (ah *ActivityHandler) handleImages(text string) (string, []clientmodels.Attachment) {
	imageURLs := getImageTagsFromHTML(text)
	var attachments []clientmodels.Attachment
	for _, imageURL := range imageURLs {
		attachments = append(attachments, clientmodels.Attachment{
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
