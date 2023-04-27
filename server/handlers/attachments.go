package handlers

import (
	"encoding/json"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-server/v6/model"
)

// handleDownloadFile handles file download
func (ah *ActivityHandler) handleDownloadFile(userID, weburl string) ([]byte, error) {
	client, err := ah.plugin.GetClientForUser(userID)
	if err != nil {
		return nil, err
	}

	data, err := client.GetFileContent(weburl)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (ah *ActivityHandler) handleAttachments(userID, channelID string, text string, msg *msteams.Message) (string, model.StringArray, string) {
	attachments := []string{}
	newText := text
	parentID := ""
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
		attachmentData, err := ah.handleDownloadFile(userID, a.ContentURL)
		if err != nil {
			ah.plugin.GetAPI().LogError("file download failed", "filename", a.Name, "error", err)
			continue
		}

		fileInfo, appErr := ah.plugin.GetAPI().UploadFile(attachmentData, channelID, a.Name)
		if appErr != nil {
			ah.plugin.GetAPI().LogError("upload file to mattermost failed", "filename", a.Name, "error", err)
			continue
		}
		attachments = append(attachments, fileInfo.Id)
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
