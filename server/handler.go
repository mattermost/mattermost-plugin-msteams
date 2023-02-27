package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/gosimple/slug"
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

func (p *Plugin) handleAttachments(channelID string, text string, msg *msteams.Message) (string, model.StringArray, string) {
	attachments := []string{}
	newText := text
	parentID := ""
	for _, a := range msg.Attachments {
		//remove the attachment tags from the text
		newText = attachRE.ReplaceAllString(newText, "")

		//handle a code snippet (code block)
		if a.ContentType == "application/vnd.microsoft.card.codesnippet" {
			newText = p.handleCodeSnippet(a, newText)
			continue
		}

		//handle a message reference (reply)
		if a.ContentType == "messageReference" {
			parentID, newText = p.handleMessageReference(a, msg.ChatID+msg.ChannelID, newText)
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
	return newText, attachments, parentID
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

// Attachments:[{ID: ContentType:messageReference Content:{\"messageId\":\"1677507519049\",\"messagePreview\":\"test\",\"messageSender\":{\"application\":null,\"device\":null,\"user\":{\"userIdentityType\":\"aadUser\",\"id\":\"2a59b646-e6ab-4f43-91a8-292bb4db599c\",\"displayName\":\"Jesus Espino\"}}} Name: ContentURL: ThumbnailURL: Data:<nil>}] ChannelID: TeamID: ChatID:19:2a59b646-e6ab-4f43-91a8-292bb4db599c_b8cf6063-314b-4a35-ad24-c127d698319b@unq.gbl.spaces}"}
func (p *Plugin) handleMessageReference(attach msteams.Attachment, chatOrChannelID string, text string) (string, string) {
	var content struct {
		MessageID string `json:"messageId"`
	}
	err := json.Unmarshal([]byte(attach.Content), &content)
	if err != nil {
		p.API.LogError("unmarshal codesnippet failed", "error", err)
		return "", text
	}
	// TODO: Make the TEAMS TO MATTERMOST POST ID dependant on the ChannelID or the ChatID to avoid collitions
	postID, err := p.store.TeamsToMattermostPostId(chatOrChannelID, content.MessageID)
	if err != nil {
		return "", text
	}

	post, err := p.API.GetPost(postID)
	if post.RootId != "" {
		return post.RootId, text
	}

	return post.Id, text
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
			token, err := p.store.GetTokenForTeamsUser(member.UserID)
			if err != nil {
				continue
			}
			client = msteams.NewTokenClient(p.configuration.TenantId, p.configuration.ClientId, token)
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
	p.API.LogDebug("MESSAGE SENT", "msg", msg)

	msgdata, _ := json.Marshal(msg)
	p.API.LogDebug("MESSAGE SENT", "msg", msg, "msgjson", string(msgdata))
	if msg.UserID == "" {
		p.API.LogDebug("Skipping not user event", "msg", msg)
		return nil
	}

	if msg.UserID == p.msteamsBotClient.BotID() {
		p.API.LogDebug("Skipping messages from bot user")
		return nil
	}

	var channelID string
	senderID := p.userID
	if chat != nil {
		userIDs := []string{}
		for _, member := range chat.Members {
			mmUserID, err := p.store.TeamsToMattermostUserId(member.UserID)
			if err != nil || mmUserID == "" {
				u, appErr := p.API.GetUserByEmail(member.UserID + "@msteamssync-plugin")
				if appErr != nil {
					var appErr2 *model.AppError
					u, appErr2 = p.API.CreateUser(&model.User{
						Username:  slug.Make(member.DisplayName) + "-" + member.UserID,
						FirstName: member.DisplayName,
						// RemoteId:  &member.UserID,
						Email:    member.UserID + "@msteamssync-plugin",
						Password: model.NewId(),
					})
					if appErr2 != nil {
						return appErr2
					}
				}
				p.store.SetTeamsToMattermostUserId(member.UserID, u.Id)
				p.store.SetMattermostToTeamsUserId(u.Id, member.UserID)
				mmUserID = u.Id
			}
			if msg.UserID == member.UserID {
				senderID = mmUserID
			}
			userIDs = append(userIDs, mmUserID)
		}
		if len(userIDs) < 2 {
			return errors.New("not enough user for creating a channel")
		}

		if chat.Type == "D" {
			p.API.LogError("CREATING CHANNEL WITH USER IDS", "user1", userIDs[0], "user2", userIDs[1])
			channel, appErr := p.API.GetDirectChannel(userIDs[0], userIDs[1])
			if appErr != nil {
				return appErr
			}
			channelID = channel.Id
		} else if chat.Type == "G" {
			channel, appErr := p.API.GetGroupChannel(userIDs)
			if appErr != nil {
				return appErr
			}
			channelID = channel.Id
		} else {
			return nil
		}
	} else {
		channelLink, _ := p.store.GetLinkByMSTeamsChannelID(msg.TeamID, msg.ChannelID)
		if channelLink != nil {
			channelID = channelLink.MattermostChannel
		}
	}

	if channelID == "" {
		p.API.LogDebug("Channel not set")
		return nil
	}

	p.API.LogDebug("Channel Obtained", "channelID", channelID)

	post, err := p.msgToPost(channelID, msg, senderID)
	if err != nil {
		p.API.LogError("Unable to transform teams post in mattermost post", "message", msg, "error", err)
		return err
	}

	p.API.LogDebug("Post generated", "post", post)

	// Avoid possible duplication
	data, _ := p.store.TeamsToMattermostPostId(msg.ChatID+msg.ChannelID, msg.ID)
	if data != "" {
		p.API.LogDebug("duplicated post")
		return nil
	}

	p.API.LogDebug("Post not duplicated")

	newPost, appErr := p.API.CreatePost(post)
	if appErr != nil {
		p.API.LogError("Unable to create post", "post", post, "error", appErr)
		return appErr
	}

	p.API.LogDebug("Post created", "post", newPost)

	if newPost != nil && newPost.Id != "" && msg.ID != "" {
		p.store.LinkPosts(newPost.Id, msg.ChatID+msg.ChannelID, msg.ID)
	}
	return nil
}

func (p *Plugin) handleUpdatedActivity(activity msteams.Activity) error {
	if activity.ClientState != p.configuration.WebhookSecret {
		p.API.LogError("Unable to process activity", "activity", activity, "error", "Invalid webhook secret")
		return errors.New("Invalid webhook secret")
	}

	// activityIds := msteams.GetActivityIds(activity)
	msg, chat, err := p.getMessageAndChatFromActivity(activity)
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

	postID, _ := p.store.TeamsToMattermostPostId(msg.ChatID+msg.ChannelID, msg.ID)
	if len(postID) == 0 {
		return nil
	}

	channelID := ""
	if chat == nil {
		channelLink, err := p.store.GetLinkByMSTeamsChannelID(msg.TeamID, msg.ChannelID)
		if err != nil || channelLink == nil {
			p.API.LogError("Unable to find the subscription")
			return errors.New("Unable to find the subscription")
		}
		channelID = channelLink.MattermostChannel
	} else {
		p, err := p.API.GetPost(postID)
		if err != nil {
			return errors.New("Unable to find the original post")
		}
		channelID = p.ChannelId
	}

	senderID, err := p.store.TeamsToMattermostUserId(msg.UserID)
	if err != nil || senderID == "" {
		senderID = p.userID
	}

	post, err := p.msgToPost(channelID, msg, senderID)
	if err != nil {
		p.API.LogError("Unable to transform teams post in mattermost post", "message", msg, "error", err)
		return err
	}

	post.Id = postID

	_, appErr := p.API.UpdatePost(post)
	if appErr != nil {
		p.API.LogError("Unable to update post", "post", post, "error", appErr)
		return appErr
	}
	return nil
}

func (p *Plugin) handleDeletedActivity(activity msteams.Activity) error {
	if activity.ClientState != p.configuration.WebhookSecret {
		p.API.LogError("Unable to process activity", "activity", activity, "error", "Invalid webhook secret")
		return errors.New("Invalid webhook secret")
	}

	activityIds := msteams.GetActivityIds(activity)

	postID, _ := p.store.TeamsToMattermostPostId(activityIds.ChatID+activityIds.ChannelID, activityIds.MessageID)
	if len(postID) == 0 {
		return nil
	}

	appErr := p.API.DeletePost(postID)
	if appErr != nil {
		p.API.LogError("Unable to to delete post", "msgID", postID, "error", appErr)
		return appErr
	}

	return nil
}

func (p *Plugin) msgToPost(channelID string, msg *msteams.Message, senderID string) (*model.Post, error) {
	text := convertToMD(msg.Text)
	props := make(map[string]interface{})
	rootID := ""

	if msg.ReplyToID != "" {
		rootID, _ = p.store.TeamsToMattermostPostId(msg.ChatID+msg.ChannelID, msg.ReplyToID)
	}

	newText, attachments, parentID := p.handleAttachments(channelID, text, msg)
	text = newText
	if parentID != "" {
		rootID = parentID
	}

	if len(rootID) == 0 && msg.Subject != "" {
		text = "## " + msg.Subject + "\n" + text
	}

	post := &model.Post{UserId: senderID, ChannelId: channelID, Message: text, Props: props, RootId: rootID, FileIds: attachments}
	post.AddProp("msteams_sync_"+p.userID, true)

	if senderID == p.userID {
		post.AddProp("override_username", msg.UserDisplayName)
		post.AddProp("override_icon_url", p.getURL()+"/avatar/"+msg.UserID)
		post.AddProp("from_webhook", "true")
	}
	return post, nil
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
