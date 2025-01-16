// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

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

	"github.com/sirupsen/logrus"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
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
			ah.plugin.GetAPI().LogWarn("failed to get image dimensions", "error", imageErr.Error())
			return "", false
		}

		imageRes := int64(width) * int64(height)
		if imageRes > *ah.plugin.GetAPI().GetConfig().FileSettings.MaxImageResolution {
			ah.plugin.GetAPI().LogWarn("image resolution is too high")
			return "", true
		}
	}

	if attachmentName == "" {
		extension := ""
		extensions, extensionErr := mime.ExtensionsByType(contentType)
		if extensionErr != nil {
			ah.plugin.GetAPI().LogWarn("Unable to get the extensions using content type", "error", extensionErr.Error())
		} else if len(extensions) > 0 {
			extension = extensions[0]
		}
		attachmentName = fmt.Sprintf("Image Pasted at %s%s", time.Now().Format("2023-01-02 15:03:05"), extension)
	}

	fileInfo, appErr := ah.plugin.GetAPI().UploadFile(attachmentData, channelID, attachmentName)
	if appErr != nil {
		ah.plugin.GetAPI().LogWarn("upload file to Mattermost failed", "filename", attachmentName, "error", appErr.Message)
		return "", false
	}

	return fileInfo.Id, false
}

func (ah *ActivityHandler) handleAttachments(channelID, userID, text string, msg *clientmodels.Message, chat *clientmodels.Chat, existingFileIDs []string) (string, model.StringArray, string, int, bool) {
	var logger logrus.FieldLogger = logrus.StandardLogger()

	logger = logger.WithFields(logrus.Fields{
		"channel_id":       channelID,
		"user_id":          userID,
		"teams_message_id": msg.ID,
	})

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
		logger.Warn("Unable to get the client to handle attachments")
		return "", nil, "", 0, errorFound
	}

	isDirectOrGroupMessage := false
	if chat != nil {
		isDirectOrGroupMessage = true
	}

	fileNames := make(map[string]string)
	if len(msg.Attachments) > 0 {
		for _, fID := range existingFileIDs {
			fileInfo, _ := ah.plugin.GetAPI().GetFileInfo(fID)
			if fileInfo != nil {
				fileNames[fileInfo.Name] = fID
			}
		}
	}

	skippedFileAttachments := 0
	for _, a := range msg.Attachments {
		logger := logger.WithFields(logrus.Fields{
			"attachment_id":           a.ID,
			"attachment_content_type": a.ContentType,
		})

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
			parentID = ah.handleMessageReference(a, msg.ChatID+msg.ChannelID)
			countNonFileAttachments++
			continue
		}

		// handle an adaptive card
		if a.ContentType == "application/vnd.microsoft.card.adaptive" {
			newText = ah.handleCard(logger, a, newText)
			countNonFileAttachments++
			continue
		}

		// The rest of the code assumes a (file) reference: ignore other content types until we explicitly support them.
		if a.ContentType != "reference" {
			logger.Warn("ignored attachment content type")
			countNonFileAttachments++
			continue
		}

		fileInfoID := fileNames[a.Name]
		if fileInfoID != "" {
			attachments = append(attachments, fileInfoID)
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
				logger.WithError(err).Warn("failed to download the file")
				ah.plugin.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonUnableToGetTeamsData, isDirectOrGroupMessage)
				skippedFileAttachments++
				continue
			}
		} else {
			fileSize, downloadURL, err = client.GetFileSizeAndDownloadURL(a.ContentURL)
			if err != nil {
				logger.WithError(err).Warn("failed to get file size and download URL")
				ah.plugin.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonUnableToGetTeamsData, isDirectOrGroupMessage)
				skippedFileAttachments++
				continue
			}

			fileSizeAllowed := *ah.plugin.GetAPI().GetConfig().FileSettings.MaxFileSize
			if fileSize > fileSizeAllowed {
				logger.WithFields(logrus.Fields{
					"file_size":         fileSize,
					"file_size_allowed": fileSizeAllowed,
				}).Warn("skipping file download from MS Teams because the file size is greater than the allowed size")
				errorFound = true
				ah.plugin.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonMaxFileSizeExceeded, isDirectOrGroupMessage)
				skippedFileAttachments++
				continue
			}

			// If the file size is less than or equal to the configurable value, then download the file directly instead of streaming
			if fileSize <= int64(ah.plugin.GetMaxSizeForCompleteDownload()*1024*1024) {
				attachmentData, err = client.GetFileContent(downloadURL)
				if err != nil {
					logger.WithError(err).Warn("failed to get file content")
					ah.plugin.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonUnableToGetTeamsData, isDirectOrGroupMessage)
					skippedFileAttachments++
					continue
				}
			}
		}

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
			ah.plugin.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonEmptyFileID, isDirectOrGroupMessage)
			skippedFileAttachments++
			continue
		}
		attachments = append(attachments, fileInfoID)
		ah.plugin.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMSTeams, "", isDirectOrGroupMessage)
		countFileAttachments++
		if countFileAttachments == maxFileAttachmentsSupported {
			// Calculate the count of file attachments discarded by subtracting handled file attachments and other attachments from total message attachments.
			fileAttachmentsDiscarded := len(msg.Attachments) - countNonFileAttachments - countFileAttachments
			ah.plugin.GetMetrics().ObserveFiles(metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonFileLimitReached, isDirectOrGroupMessage, int64(fileAttachmentsDiscarded))
			skippedFileAttachments += fileAttachmentsDiscarded
			break
		}
	}

	return newText, attachments, parentID, skippedFileAttachments, errorFound
}

func (ah *ActivityHandler) GetFileFromTeamsAndUploadToMM(downloadURL string, client msteams.Client, us *model.UploadSession) string {
	pipeReader, pipeWriter := io.Pipe()
	uploadSession, err := ah.plugin.GetAPI().CreateUploadSession(us)
	if err != nil {
		ah.plugin.GetAPI().LogWarn("Unable to create an upload session in Mattermost", "error", err.Error())
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
		ah.plugin.GetAPI().LogWarn("Unable to upload data in the upload session", "upload_session_id", uploadSession.Id, "error", err.Error())
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
		ah.plugin.GetAPI().LogWarn("failed to unmarshal codesnippet", "error", err.Error())
		return text
	}
	s := strings.Split(content.CodeSnippetURL, "/")
	if !strings.Contains(content.CodeSnippetURL, "chats") && !strings.Contains(content.CodeSnippetURL, "channels") {
		ah.plugin.GetAPI().LogWarn("invalid codesnippetURL", "URL", content.CodeSnippetURL)
		return text
	}

	if (strings.Contains(content.CodeSnippetURL, "chats") && len(s) != 11) || (strings.Contains(content.CodeSnippetURL, "channels") && len(s) != 13 && len(s) != 15) {
		ah.plugin.GetAPI().LogWarn("codesnippetURL has unexpected size", "URL", content.CodeSnippetURL)
		return text
	}

	codeSnippetText, err := client.GetCodeSnippet(content.CodeSnippetURL)
	if err != nil {
		ah.plugin.GetAPI().LogWarn("retrieving snippet content failed", "error", err)
		return text
	}
	newText := text + "\n```" + content.Language + "\n" + codeSnippetText + "\n```\n"
	return newText
}

func (ah *ActivityHandler) handleMessageReference(attach clientmodels.Attachment, chatOrChannelID string) string {
	var content struct {
		MessageID string `json:"messageId"`
	}
	err := json.Unmarshal([]byte(attach.Content), &content)
	if err != nil {
		ah.plugin.GetAPI().LogWarn("failed to unmarshal attachment content", "error", err)
		return ""
	}
	postInfo, err := ah.plugin.GetStore().GetPostInfoByMSTeamsID(chatOrChannelID, content.MessageID)
	if err != nil {
		return ""
	}

	post, appErr := ah.plugin.GetAPI().GetPost(postInfo.MattermostID)
	if appErr != nil {
		return ""
	}

	if post.RootId != "" {
		return post.RootId
	}

	return post.Id
}

func (ah *ActivityHandler) handleCard(logger logrus.FieldLogger, attach clientmodels.Attachment, text string) string {
	var content struct {
		Type string `json:"type"`
		Body []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		} `json:"body"`
	}
	err := json.Unmarshal([]byte(attach.Content), &content)
	if err != nil {
		logger.WithError(err).Warn("failed to unmarshal card")
		return text
	}

	logger = logger.WithField("card_type", content.Type)

	if content.Type != "AdaptiveCard" {
		logger.Warn("ignoring unexpected card type")
		return text
	}

	foundContent := false
	for _, element := range content.Body {
		if element.Type == "TextBlock" {
			foundContent = true
			text = text + "\n" + element.Text
			continue
		}

		logger.Debug("skipping unsupported element type in card", "element_type", element.Type)
	}

	if !foundContent {
		logger.Warn("failed to find any text to render from card")
	}

	return text
}
