package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-server/v6/app/imaging"
	"github.com/mattermost/mattermost-server/v6/model"
)

// handleDownloadFile handles file download
func (ah *ActivityHandler) handleDownloadFile(weburl string, chat *msteams.Chat) ([]byte, error) {
	var client msteams.Client
	if chat != nil {
		for _, member := range chat.Members {
			client, _ = ah.plugin.GetClientForTeamsUser(member.UserID)
			if client != nil {
				break
			}
		}

	} else {
		client = ah.plugin.GetClientForApp()
	}

	if client == nil {
		return nil, errors.New("unable to get the client")
	}

	data, err := client.GetFileContent(weburl)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// TODO: Add unit tests for this function
func (ah *ActivityHandler) handleAttachments(userID, channelID string, text string, msg *msteams.Message, chat *msteams.Chat) (string, model.StringArray, string) {
	attachments := []string{}
	newText := text
	parentID := ""
	countAttachments := 0
	for _, a := range msg.Attachments {
		// remove the attachment tags from the text
		newText = attachRE.ReplaceAllString(newText, "")

		// handle a code snippet (code block)
		if a.ContentType == "application/vnd.microsoft.card.codesnippet" {
			newText = ah.handleCodeSnippet(userID, a, newText)
			continue
		}

		// handle a message reference (reply)
		if a.ContentType == "messageReference" {
			parentID, newText = ah.handleMessageReference(a, msg.ChatID+msg.ChannelID, newText)
			continue
		}

		// handle the download
		attachmentData, err := ah.handleDownloadFile(a.ContentURL, chat)
		if err != nil {
			ah.plugin.GetAPI().LogError("file download failed", "filename", a.Name, "error", err)
			continue
		}

		fileSizeAllowed := *ah.plugin.GetAPI().GetConfig().FileSettings.MaxFileSize
		if len(attachmentData) > int(fileSizeAllowed) {
			ah.plugin.GetAPI().LogError("cannot upload file to mattermost as its size is greater than allowed size", "filename", a.Name)
			continue
		}

		contentType := http.DetectContentType(attachmentData)
		if strings.HasPrefix(contentType, "image") && contentType != "image/svg+xml" {
			w, h, imageErr := imaging.GetDimensions(bytes.NewReader(attachmentData))
			if imageErr != nil {
				ah.plugin.GetAPI().LogError("failed to get image dimensions", "error", imageErr.Error())
				continue
			}

			imageRes := int64(w) * int64(h)
			if imageRes > *ah.plugin.GetAPI().GetConfig().FileSettings.MaxImageResolution {
				ah.plugin.GetAPI().LogError("image resolution is too high")
				continue
			}
		}

		fileInfo, appErr := ah.plugin.GetAPI().UploadFile(attachmentData, channelID, a.Name)
		if appErr != nil {
			ah.plugin.GetAPI().LogError("upload file to mattermost failed", "filename", a.Name, "error", err)
			continue
		}

		attachments = append(attachments, fileInfo.Id)
		countAttachments++
		if countAttachments == 10 {
			ah.plugin.GetAPI().LogDebug("discarding the rest of the attachments as mattermost supports only 10 attachments per post")
			break
		}
	}
	return newText, attachments, parentID
}

func (ah *ActivityHandler) handleCodeSnippet(userID string, attach msteams.Attachment, text string) string {
	var content struct {
		Language       string `json:"language"`
		CodeSnippetURL string `json:"codeSnippetUrl"`
	}
	err := json.Unmarshal([]byte(attach.Content), &content)
	if err != nil {
		ah.plugin.GetAPI().LogError("unmarshal codesnippet failed", "error", err)
		return text
	}
	s := strings.Split(content.CodeSnippetURL, "/")
	if len(s) != 13 && len(s) != 15 {
		ah.plugin.GetAPI().LogError("codesnippetUrl has unexpected size", "size", content.CodeSnippetURL)
		return text
	}

	client, err := ah.plugin.GetClientForUser(userID)
	if err != nil {
		ah.plugin.GetAPI().LogError("unable to get bot client", "error", err)
		return text
	}

	codeSnippetText, err := client.GetCodeSnippet(content.CodeSnippetURL)
	if err != nil {
		ah.plugin.GetAPI().LogError("retrieving snippet content failed", "error", err)
		return text
	}
	newText := text + "\n```" + content.Language + "\n" + codeSnippetText + "\n```\n"
	return newText
}

func (ah *ActivityHandler) handleMessageReference(attach msteams.Attachment, chatOrChannelID string, text string) (string, string) {
	var content struct {
		MessageID string `json:"messageId"`
	}
	err := json.Unmarshal([]byte(attach.Content), &content)
	if err != nil {
		ah.plugin.GetAPI().LogError("unmarshal codesnippet failed", "error", err)
		return "", text
	}
	postInfo, err := ah.plugin.GetStore().GetPostInfoByMSTeamsID(chatOrChannelID, content.MessageID)
	if err != nil {
		return "", text
	}

	post, appErr := ah.plugin.GetAPI().GetPost(postInfo.MattermostID)
	if appErr != nil {
		return "", text
	}

	if post.RootId != "" {
		return post.RootId, text
	}

	return post.Id, text
}
