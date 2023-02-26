package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattn/godown"
	"github.com/pkg/errors"
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
			p.API.LogError("file download failed", "filename", a.Name, "error", err)
			continue
		}

		fileInfo, appErr := p.API.UploadFile(attachmentData, channelID, a.Name)
		if appErr != nil {
			p.API.LogError("upload file to mattermost failed", "filename", a.Name, "error", err)
			continue
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
		p.API.LogError("unmarshal codesnippet failed", "error", err)
		return text
	}
	s := strings.Split(content.CodeSnippetURL, "/")
	if len(s) != 13 && len(s) != 15 {
		p.API.LogError("codesnippetUrl has unexpected size", "size", content.CodeSnippetURL)
		return text
	}
	codeSnippetText, err := p.msteamsBotClient.GetCodeSnippet(content.CodeSnippetURL)
	if err != nil {
		p.API.LogError("retrieving snippet content failed", "error", err)
		return text
	}
	newText := text + "\n```" + content.Language + "\n" + codeSnippetText + "\n```\n"
	return newText
}

func (p *Plugin) getMessageAndChatFromActivity(activity msteams.Activity) (*msteams.Message, *msteams.Chat, error) {
	activityIds := msteams.GetActivityIds(activity)

	if activityIds.ChatID != "" {
		chat, err := p.msteamsAppClient.GetChat(activityIds.ChatID)
		if err != nil {
			return nil, nil, err
		}
		var client msteams.Client
		for _, member := range chat.Members {
			token, err := p.store.GetTokenForMattermostUser(member.UserID)
			if err != nil {
				continue
			}
			client = msteams.NewTokenClient(token)
		}
		if client == nil {
			// TODO: None of the users are connected to MSTeams, ignoring the message
			return nil, nil, nil
		}

		msg, err := client.GetChatMessage(activityIds.ChatID, activityIds.MessageID)
		if err != nil {
			p.API.LogError("Unable to get original post", "error", err)
			return nil, nil, err
		}
		return msg, chat, nil
	}

	if activityIds.ReplyID != "" {
		msg, err := p.msteamsBotClient.GetReply(activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID, activityIds.ReplyID)
		if err != nil {
			p.API.LogError("Unable to get original post", "error", err)
			return nil, nil, err
		}
		return msg, nil, nil
	}

	msg, err := p.msteamsBotClient.GetMessage(activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID)
	if err != nil {
		p.API.LogError("Unable to get original post", "error", err)
		return nil, nil, err
	}
	return msg, nil, nil
}

func (p *Plugin) handleCreatedActivity(activity msteams.Activity) error {
	if activity.ClientState != p.configuration.WebhookSecret {
		p.API.LogError("Unable to process activity", "activity", activity, "error", "Invalid webhook secret")
		return errors.New("Invalid webhook secret")
	}

	msg, chat, err := p.getMessageAndChatFromActivity(activity)
	if err != nil {
		return err
	}

	if msg == nil {
		p.API.LogDebug("Unable to get the message (probably because belongs to private chate in not-linked users)")
		return nil
	}

	if msg.UserID == "" {
		p.API.LogDebug("Skipping not user event", "msg", msg)
		return nil
	}

	if msg.UserID == p.msteamsBotClient.BotID() {
		p.API.LogDebug("Skipping messages from bot user")
		return nil
	}

	var channel *model.Channel
	if chat != nil {
		userIDs := []string{}
		for _, member := range chat.Members {
			u, appErr := p.API.GetUserByEmail(member.Email)
			if appErr != nil {
				// TODO: Check if the error is a doesn't exists
				var appErr2 *model.AppError
				u, appErr2 = p.API.CreateUser(&model.User{
					Username:  model.NewId(),
					FirstName: member.DisplayName,
					Email:     member.Email,
					Password:  model.NewId(),
				})
				if appErr2 != nil {
					return appErr2
				}
			}
			userIDs = append(userIDs, u.Id)
		}
		if len(userIDs) < 2 {
			return errors.New("not enough user for creating a channel")
		}

		var appErr *model.AppError
		if chat.Type == "D" {
			channel, appErr = p.API.GetDirectChannel(userIDs[0], userIDs[1])
			if appErr != nil {
				return appErr
			}
		} else if chat.Type == "G" {
			channel, appErr = p.API.GetGroupChannel(userIDs)
			if appErr != nil {
				return appErr
			}
		} else {
			return nil
		}
	} else {
		channelLink, _ := p.store.GetLinkByMSTeamsChannelID(msg.TeamID, msg.ChannelID)
		if channelLink != nil {
			var appErr *model.AppError
			channel, appErr = p.API.GetChannel(channelLink.MattermostChannel)
			if appErr != nil {
				p.API.LogError("Unable to get the channel", "error", appErr)
				return appErr
			}
		}
	}

	if channel == nil {
		p.API.LogDebug("Channel not set")
		return nil
	}

	post, err := p.msgToPost(channel, msg)
	if err != nil {
		p.API.LogError("Unable to transform teams post in mattermost post", "message", msg, "error", err)
		return err
	}

	// Avoid possible duplication
	data, _ := p.store.TeamsToMattermostPostId(msg.ID)
	if data != "" {
		p.API.LogDebug("duplicated post")
		return nil
	}

	newPost, appErr := p.API.CreatePost(post)
	if appErr != nil {
		p.API.LogError("Unable to create post", "post", post, "error", appErr)
		return appErr
	}

	if newPost != nil && newPost.Id != "" && msg.ID != "" {
		p.store.LinkPosts(newPost.Id, msg.ID)
	}
	return nil
}

func (p *Plugin) handleUpdatedActivity(activity msteams.Activity) error {
	if activity.ClientState != p.configuration.WebhookSecret {
		p.API.LogError("Unable to process activity", "activity", activity, "error", "Invalid webhook secret")
		return errors.New("Invalid webhook secret")
	}

	// activityIds := msteams.GetActivityIds(activity)
	msg, _, err := p.getMessageAndChatFromActivity(activity)
	if err != nil {
		return err
	}

	if msg.UserID == "" {
		p.API.LogDebug("Skipping not user event", "msg", msg)
		return nil
	}

	if msg.UserID == p.msteamsBotClient.BotID() {
		p.API.LogDebug("Skipping messages from bot user")
		return nil
	}

	// channelLink := p.links.GetLinkBySubscriptionID(activity.SubscriptionId)
	// if channelLink == nil {
	// 	p.API.LogError("Unable to find the subscription")
	// 	return errors.New("Unable to find the subscription")
	// }

	// post, err := p.msgToPost(channelLink, msg)
	// if err != nil {
	// 	p.API.LogError("Unable to transform teams post in mattermost post", "message", msg, "error", err)
	// 	return err
	// }

	// postID, _ := p.store.TeamsToMattermostPostId(msg.ID)
	// if len(postID) == 0 {
	// 	return nil
	// }
	// post.Id = postID

	// _, appErr := p.API.UpdatePost(post)
	// if appErr != nil {
	// 	p.API.LogError("Unable to update post", "post", post, "error", appErr)
	// 	return appErr
	// }
	return nil
}

func (p *Plugin) handleDeletedActivity(activity msteams.Activity) error {
	// activityIds := msteams.GetActivityIds(activity)
	if activity.ClientState != p.configuration.WebhookSecret {
		p.API.LogError("Unable to process activity", "activity", activity, "error", "Invalid webhook secret")
		return errors.New("Invalid webhook secret")
	}

	// channelLink := p.links.GetLinkBySubscriptionID(activity.SubscriptionId)
	// if channelLink == nil {
	// 	p.API.LogError("Unable to find the subscription")
	// 	return errors.New("Unable to find the subscription")
	// }

	// msgID := activityIds.ReplyID
	// if msgID == "" {
	// 	msgID = activityIds.MessageID
	// }

	// data, _ := p.store.TeamsToMattermostPostId(msgID)
	// if len(data) == 0 {
	// 	p.API.LogError("Unable to find original post", "msgID", msgID)
	// 	return nil
	// }

	// appErr := p.API.DeletePost(string(data))
	// if appErr != nil {
	// 	p.API.LogError("Unable to to delete post", "msgID", string(data), "error", appErr)
	// 	return appErr
	// }

	return nil
}

func (p *Plugin) msgToPost(channel *model.Channel, msg *msteams.Message) (*model.Post, error) {
	text := convertToMD(msg.Text)
	props := make(map[string]interface{})
	rootID := ""

	if msg.ReplyToID != "" {
		rootID, _ = p.store.TeamsToMattermostPostId(msg.ReplyToID)
	}

	newText, attachments := p.handleAttachments(channel.Id, text, msg)
	text = newText

	if len(rootID) == 0 && msg.Subject != "" {
		text = "## " + msg.Subject + "\n" + text
	}

	post := &model.Post{UserId: p.userID, ChannelId: channel.Id, Message: text, Props: props, RootId: rootID, FileIds: attachments}
	post.AddProp("msteams_sync_"+p.userID, true)
	post.AddProp("override_username", msg.UserDisplayName)
	post.AddProp("override_icon_url", p.getURL()+"/avatar/"+msg.UserID)
	post.AddProp("from_webhook", "true")
	return post, nil
}

func convertToMD(text string) string {
	if !strings.Contains(text, "<div>") {
		return text
	}
	var sb strings.Builder
	err := godown.Convert(&sb, strings.NewReader(text), nil)
	if err != nil {
		return text
	}
	return sb.String()
}
