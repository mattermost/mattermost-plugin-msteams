package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-plugin-matterbridge/server/msteams"
	"github.com/mattermost/mattermost-server/v6/model"
)

var attachRE = regexp.MustCompile(`<attachment id=.*?attachment>`)

// handleDownloadFile handles file download
func (p *Plugin) handleDownloadFile(filename, weburl string) ([]byte, error) {
	realURL, err := p.msteamsBotClient.GetFileURL(weburl)
	if err != nil {
		return nil, err
	}
	// Actually download the file.
	res, err := http.DefaultClient.Get(realURL)
	if err != nil {
		return nil, fmt.Errorf("download %s failed %#v", weburl, err)
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("download %s failed %#v", weburl, err)
	}

	return data, nil
}

func (p *Plugin) handleAttachments(channelID string, text string, msg *msteams.Message) (string, model.StringArray) {
	attachments := []string{}
	newText := text
	for _, a := range msg.Attachments {
		//remove the attachment tags from the text
		newText = attachRE.ReplaceAllString(newText, "")

		//handle a code snippet (code block)
		if a.ContentType == "application/vnd.microsoft.card.codesnippet" {
			newText = p.handleCodeSnippet(a, newText)
			continue
		}

		//handle the download
		attachmentData, err := p.handleDownloadFile(a.Name, a.ContentURL)
		if err != nil {
			p.API.LogError("download of %s failed: %s", a.Name, err)
			continue
		}

		fileInfo, appErr := p.API.UploadFile(attachmentData, channelID, a.Name)
		if appErr != nil {
			p.API.LogError("upload file to mattermost failed", "filename", a.Name, "error", err)
		}
		attachments = append(attachments, fileInfo.Id)
	}
	return newText, attachments
}

func (p *Plugin) handleCodeSnippet(attach msteams.Attachment, text string) string {
	var content struct {
		Language       string `json:"language"`
		CodeSnippetURL string `json:"codeSnippetUrl"`
	}
	err := json.Unmarshal([]byte(attach.Content), &content)
	if err != nil {
		p.API.LogError("unmarshal codesnippet failed: %s", err)
		return text
	}
	s := strings.Split(content.CodeSnippetURL, "/")
	if len(s) != 13 && len(s) != 15 {
		p.API.LogError("codesnippetUrl has unexpected size", "size", content.CodeSnippetURL)
		return text
	}
	codeSnippetText, err := p.msteamsBotClient.GetCodeSnippet(content.CodeSnippetURL)
	if err != nil {
		p.API.LogError("retrieving snippet content failed:%s", err)
		return text
	}
	newText := text + "\n```" + content.Language + "\n" + codeSnippetText + "\n```\n"
	return newText
}
