package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app/imaging"
)

func GetResourceIDsFromURL(weburl string) (*clientmodels.ActivityIds, error) {
	parsedURL, err := url.Parse(weburl)
	if err != nil {
		return nil, err
	}

	path := strings.TrimPrefix(parsedURL.Path, "/beta/")
	path = strings.TrimPrefix(path, "/v1.0/")
	urlParts := strings.Split(path, "/")
	activityIDs := &clientmodels.ActivityIds{}
	if urlParts[0] == "chats" && len(urlParts) >= 6 {
		activityIDs.ChatID = urlParts[1]
		activityIDs.MessageID = urlParts[3]
		activityIDs.HostedContentsID = urlParts[5]
	} else if len(urlParts) >= 6 {
		activityIDs.TeamID = urlParts[1]
		activityIDs.ChannelID = urlParts[3]
		activityIDs.MessageID = urlParts[5]
		if strings.Contains(path, "replies") && len(urlParts) >= 10 {
			activityIDs.ReplyID = urlParts[7]
			activityIDs.HostedContentsID = urlParts[9]
		} else {
			activityIDs.HostedContentsID = urlParts[7]
		}
	}

	return activityIDs, nil
}

// handleDownloadFile handles file download
func (ah *ActivityHandler) handleDownloadFile(weburl string, client msteams.Client) ([]byte, error) {
	activityIDs, err := GetResourceIDsFromURL(weburl)
	if err != nil {
		return nil, err
	}

	return client.GetHostedFileContent(activityIDs)
}

func (ah *ActivityHandler) ProcessAndUploadFileToMM(attachmentData []byte, attachmentName, channelID string) (fileInfoID string, resolutionErrorFound bool) {
	contentType := http.DetectContentType(attachmentData)
	if strings.HasPrefix(contentType, "image") && contentType != "image/svg+xml" {
		width, height, imageErr := imaging.GetDimensions(bytes.NewReader(attachmentData))
		if imageErr != nil {
			ah.plugin.GetAPI().LogError("failed to get image dimensions", "error", imageErr.Error())
			return "", false
		}

		imageRes := int64(width) * int64(height)
		if imageRes > *ah.plugin.GetAPI().GetConfig().FileSettings.MaxImageResolution {
			ah.plugin.GetAPI().LogError("image resolution is too high")
			return "", true
		}
	}

	if attachmentName == "" {
		extension := ""
		extensions, extensionErr := mime.ExtensionsByType(contentType)
		if extensionErr != nil {
			ah.plugin.GetAPI().LogDebug("Unable to get the extensions using content type", "error", extensionErr.Error())
		} else if len(extensions) > 0 {
			extension = extensions[0]
		}
		attachmentName = fmt.Sprintf("Image Pasted at %s%s", time.Now().Format("2023-01-02 15:03:05"), extension)
	}

	fileInfo, appErr := ah.plugin.GetAPI().UploadFile(attachmentData, channelID, attachmentName)
	if appErr != nil {
		ah.plugin.GetAPI().LogError("upload file to Mattermost failed", "filename", attachmentName, "error", appErr.Message)
		return "", false
	}

	return fileInfo.Id, false
}

func (ah *ActivityHandler) handleAttachments(channelID, userID, text string, msg *clientmodels.Message, chat *clientmodels.Chat, isUpdatedActivity bool) (string, model.StringArray, string, bool) {
	attachments := []string{}
	newText := text
	parentID := ""
	countNonFileAttachments := 0
	countFileAttachments := 0
	var client msteams.Client
	if chat == nil {
		client = ah.plugin.GetClientForApp()
	} else {
		for _, member := range chat.Members {
			client, _ = ah.plugin.GetClientForTeamsUser(member.UserID)
			if client != nil {
				break
			}
		}
	}

	errorFound := false
	if client == nil {
		ah.plugin.GetAPI().LogError("Unable to get the client")
		return "", nil, "", errorFound
	}

	isDirectMessage := false
	if chat != nil {
		isDirectMessage = true
	}

	for _, a := range msg.Attachments {
		// remove the attachment tags from the text
		newText = attachRE.ReplaceAllString(newText, "")

		// handle a code snippet (code block)
		if a.ContentType == "application/vnd.microsoft.card.codesnippet" {
			newText = ah.handleCodeSnippet(client, a, newText)
			countNonFileAttachments++
			continue
		}

		// handle a message reference (reply)
		if a.ContentType == "messageReference" {
			parentID, newText = ah.handleMessageReference(a, msg.ChatID+msg.ChannelID, newText)
			countNonFileAttachments++
			continue
		}

		if isUpdatedActivity {
			continue
		}

		// handle the download
		var attachmentData []byte
		var err error
		var fileSize int64
		downloadURL := ""
		if strings.Contains(a.ContentURL, hostedContentsStr) && strings.HasSuffix(a.ContentURL, "$value") {
			attachmentData, err = ah.handleDownloadFile(a.ContentURL, client)
			if err != nil {
				ah.plugin.GetAPI().LogError("failed to download the file", "filename", a.Name, "error", err.Error())
				ah.plugin.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonUnableToGetTeamsData, isDirectMessage)
				continue
			}
		} else {
			fileSize, downloadURL, err = client.GetFileSizeAndDownloadURL(a.ContentURL)
			if err != nil {
				ah.plugin.GetAPI().LogError("failed to get file size and download URL", "error", err.Error())
				ah.plugin.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonUnableToGetTeamsData, isDirectMessage)
				continue
			}

			fileSizeAllowed := *ah.plugin.GetAPI().GetConfig().FileSettings.MaxFileSize
			if fileSize > fileSizeAllowed {
				ah.plugin.GetAPI().LogError("skipping file download from MS Teams because the file size is greater than the allowed size")
				errorFound = true
				ah.plugin.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonMaxFileSizeExceeded, isDirectMessage)
				continue
			}

			// If the file size is less than or equal to the configurable value, then download the file directly instead of streaming
			if fileSize <= int64(ah.plugin.GetMaxSizeForCompleteDownload()*1024*1024) {
				attachmentData, err = client.GetFileContent(downloadURL)
				if err != nil {
					ah.plugin.GetAPI().LogError("failed to get file content", "error", err.Error())
					ah.plugin.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonUnableToGetTeamsData, isDirectMessage)
					continue
				}
			}
		}

		fileInfoID := ""
		if attachmentData != nil {
			fileInfoID, errorFound = ah.ProcessAndUploadFileToMM(attachmentData, a.Name, channelID)
		} else {
			fileInfoID = ah.GetFileFromTeamsAndUploadToMM(downloadURL, client, &model.UploadSession{
				Id:        model.NewId(),
				Type:      model.UploadTypeAttachment,
				ChannelId: channelID,
				UserId:    userID,
				Filename:  a.Name,
				FileSize:  fileSize,
			})
		}

		if fileInfoID == "" {
			ah.plugin.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonEmptyFileID, isDirectMessage)
			continue
		}
		attachments = append(attachments, fileInfoID)
		ah.plugin.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMSTeams, "", isDirectMessage)
		countFileAttachments++
		if countFileAttachments == maxFileAttachmentsSupported {
			ah.plugin.GetAPI().LogDebug("discarding the rest of the attachments as Mattermost supports only 10 attachments per post")

			// Calculate the count of file attachments discarded by subtracting handled file attachments and other attachments from total message attachments.
			fileAttachmentsDiscarded := len(msg.Attachments) - countNonFileAttachments - countFileAttachments
			ah.plugin.GetMetrics().ObserveFiles(metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonFileLimitReached, isDirectMessage, int64(fileAttachmentsDiscarded))
			break
		}
	}

	return newText, attachments, parentID, errorFound
}

func (ah *ActivityHandler) GetFileFromTeamsAndUploadToMM(downloadURL string, client msteams.Client, us *model.UploadSession) string {
	pipeReader, pipeWriter := io.Pipe()
	uploadSession, err := ah.plugin.GetAPI().CreateUploadSession(us)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to create an upload session in Mattermost", "error", err.Error())
		return ""
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				ah.plugin.GetMetrics().ObserveGoroutineFailure()
				ah.plugin.GetAPI().LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
			}
		}()

		client.GetFileContentStream(downloadURL, pipeWriter, int64(ah.plugin.GetBufferSizeForStreaming()*1024*1024))
	}()
	fileInfo, err := ah.plugin.GetAPI().UploadData(uploadSession, pipeReader)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to upload data in the upload session", "UploadSessionID", uploadSession.Id, "Error", err.Error())
		return ""
	}

	return fileInfo.Id
}

func (ah *ActivityHandler) handleCodeSnippet(client msteams.Client, attach clientmodels.Attachment, text string) string {
	var content struct {
		Language       string `json:"language"`
		CodeSnippetURL string `json:"codeSnippetUrl"`
	}
	err := json.Unmarshal([]byte(attach.Content), &content)
	if err != nil {
		ah.plugin.GetAPI().LogError("failed to unmarshal codesnippet", "error", err.Error())
		return text
	}
	s := strings.Split(content.CodeSnippetURL, "/")
	if !strings.Contains(content.CodeSnippetURL, "chats") && !strings.Contains(content.CodeSnippetURL, "channels") {
		ah.plugin.GetAPI().LogError("invalid codesnippetURL", "URL", content.CodeSnippetURL)
		return text
	}

	if (strings.Contains(content.CodeSnippetURL, "chats") && len(s) != 11) || (strings.Contains(content.CodeSnippetURL, "channels") && len(s) != 13 && len(s) != 15) {
		ah.plugin.GetAPI().LogError("codesnippetURL has unexpected size", "URL", content.CodeSnippetURL)
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

func (ah *ActivityHandler) handleMessageReference(attach clientmodels.Attachment, chatOrChannelID string, text string) (string, string) {
	var content struct {
		MessageID string `json:"messageId"`
	}
	err := json.Unmarshal([]byte(attach.Content), &content)
	if err != nil {
		ah.plugin.GetAPI().LogError("failed to unmarshal attachment content", "error", err)
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
