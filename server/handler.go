package main

import (
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattn/godown"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
)

var emojisReverseMap map[string]string

var attachRE = regexp.MustCompile(`<attachment id=.*?attachment>`)

func (p *Plugin) handleActivity(activity msteams.Activity) error {
	if activity.ClientState != p.configuration.WebhookSecret {
		p.API.LogError("Unable to process activity", "activity", activity, "error", "Invalid webhook secret")
		return errors.New("Invalid webhook secret")
	}

	activityIds := msteams.GetResourceIds(activity.Resource)

	// TODO: Make this something that has a limit on the number of goroutines
	switch activity.ChangeType {
	case "created":
		p.API.LogDebug("Handling create activity", "activity", activity)
		go p.handleCreatedActivity(activityIds)
	case "updated":
		p.API.LogDebug("Handling update activity", "activity", activity)
		go p.handleUpdatedActivity(activityIds)
	case "deleted":
		p.API.LogDebug("Handling delete activity", "activity", activity)
		go p.handleDeletedActivity(activityIds)
	default:
		p.API.LogWarn("Unandledy activity", "activity", activity, "error", "Not handled activity")
	}

	return nil
}

// handleDownloadFile handles file download
func (p *Plugin) handleDownloadFile(filename, weburl string) ([]byte, error) {
	client, err := p.getClientForUser(p.userID)
	if err != nil {
		return nil, err
	}

	realURL, err := client.GetFileURL(weburl)
	if err != nil {
		return nil, err
	}
	// Actually download the file.
	res, err := http.DefaultClient.Get(realURL)
	if err != nil {
		return nil, fmt.Errorf("download %s failed %#v", weburl, err)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
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
		// remove the attachment tags from the text
		newText = attachRE.ReplaceAllString(newText, "")

		// handle a code snippet (code block)
		if a.ContentType == "application/vnd.microsoft.card.codesnippet" {
			newText = p.handleCodeSnippet(a, newText)
			continue
		}

		// handle a message reference (reply)
		if a.ContentType == "messageReference" {
			parentID, newText = p.handleMessageReference(a, msg.ChatID+msg.ChannelID, newText)
			continue
		}

		// handle the download
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

	client, err := p.getClientForUser(p.userID)
	if err != nil {
		p.API.LogError("unable to get bot client", "error", err)
		return text
	}

	codeSnippetText, err := client.GetCodeSnippet(content.CodeSnippetURL)
	if err != nil {
		p.API.LogError("retrieving snippet content failed", "error", err)
		return text
	}
	newText := text + "\n```" + content.Language + "\n" + codeSnippetText + "\n```\n"
	return newText
}

func (p *Plugin) handleMessageReference(attach msteams.Attachment, chatOrChannelID string, text string) (string, string) {
	var content struct {
		MessageID string `json:"messageId"`
	}
	err := json.Unmarshal([]byte(attach.Content), &content)
	if err != nil {
		p.API.LogError("unmarshal codesnippet failed", "error", err)
		return "", text
	}
	postInfo, err := p.store.GetPostInfoByMSTeamsID(chatOrChannelID, content.MessageID)
	if err != nil {
		return "", text
	}

	post, appErr := p.API.GetPost(postInfo.MattermostID)
	if appErr != nil {
		return "", text
	}

	if post.RootId != "" {
		return post.RootId, text
	}

	return post.Id, text
}

func (p *Plugin) getMessageFromChat(chat *msteams.Chat, messageID string) (*msteams.Message, error) {
	var client msteams.Client
	for _, member := range chat.Members {
		client, _ = p.getClientForTeamsUser(member.UserID)
		if client != nil {
			break
		}
	}
	if client == nil {
		return nil, nil
	}

	msg, err := client.GetChatMessage(chat.ID, messageID)
	if err != nil {
		p.API.LogError("Unable to get original post", "error", err)
		return nil, err
	}
	return msg, nil
}

func (p *Plugin) getReplyFromChannel(teamID, channelID, messageID, replyID string) (*msteams.Message, error) {
	client, err := p.getClientForUser(p.userID)
	if err != nil {
		p.API.LogError("unable to get bot client", "error", err)
		return nil, err
	}

	var msg *msteams.Message
	msg, err = client.GetReply(teamID, channelID, messageID, replyID)
	if err != nil {
		p.API.LogError("Unable to get original post", "error", err)
		return nil, err
	}
	return msg, nil
}

func (p *Plugin) getMessageFromChannel(teamID, channelID, messageID string) (*msteams.Message, error) {
	client, err := p.getClientForUser(p.userID)
	if err != nil {
		p.API.LogError("unable to get bot client", "error", err)
		return nil, err
	}

	msg, err := client.GetMessage(teamID, channelID, messageID)
	if err != nil {
		p.API.LogError("Unable to get original post", "error", err)
		return nil, err
	}
	return msg, nil
}

func (p *Plugin) getMessageAndChatFromActivityIds(activityIds msteams.ActivityIds) (*msteams.Message, *msteams.Chat, error) {
	var msg *msteams.Message
	var chat *msteams.Chat
	if activityIds.ChatID != "" {
		var err error
		chat, err = p.msteamsAppClient.GetChat(activityIds.ChatID)
		if err != nil {
			p.API.LogError("Unable to get original chat", "error", err.Error())
			return nil, nil, err
		}
		msg, err = p.getMessageFromChat(chat, activityIds.MessageID)
		if err != nil {
			p.API.LogError("Unable to get original message", "error", err.Error())
			return nil, nil, err
		}
	} else if activityIds.ReplyID != "" {
		var err error
		msg, err = p.getReplyFromChannel(activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID, activityIds.ReplyID)
		if err != nil {
			p.API.LogError("Unable to get original post", "error", err)
			return nil, nil, err
		}
	} else {
		var err error
		msg, err = p.getMessageFromChannel(activityIds.TeamID, activityIds.ChannelID, activityIds.MessageID)
		if err != nil {
			p.API.LogError("Unable to get original post", "error", err)
			return nil, nil, err
		}
	}
	return msg, chat, nil
}

func (p *Plugin) handleCreatedActivity(activityIds msteams.ActivityIds) {
	msg, chat, err := p.getMessageAndChatFromActivityIds(activityIds)
	if err != nil {
		p.API.LogError("Unable to get original message", "error", err.Error())
		return
	}

	if msg == nil {
		p.API.LogDebug("Unable to get the message (probably because belongs to private chate in not-linked users)")
		return
	}

	if msg.UserID == "" {
		p.API.LogDebug("Skipping not user event", "msg", msg)
		return
	}

	msteamsUserID, _ := p.store.MattermostToTeamsUserID(p.userID)
	if msg.UserID == msteamsUserID {
		p.API.LogDebug("Skipping messages from bot user")
		p.updateLastReceivedChangeDate(msg.LastUpdateAt)
		return
	}

	var channelID string
	senderID, err := p.store.TeamsToMattermostUserID(msg.UserID)
	if err != nil || senderID == "" {
		senderID = p.userID
	}
	if chat != nil {
		if !p.configuration.SyncDirectMessages {
			// Skipping because direct/group messages are disabled
			return
		}
		channelID, err = p.getChatChannelID(chat, msg.UserID)
		if err != nil {
			p.API.LogError("Unable to get original channel id", "error", err.Error())
			return
		}
	} else {
		channelLink, _ := p.store.GetLinkByMSTeamsChannelID(msg.TeamID, msg.ChannelID)
		if channelLink != nil {
			channelID = channelLink.MattermostChannel
		}
	}

	if channelID == "" {
		p.API.LogDebug("Channel not set")
		return
	}

	post, err := p.msgToPost(channelID, msg, senderID)
	if err != nil {
		p.API.LogError("Unable to transform teams post in mattermost post", "message", msg, "error", err)
		return
	}

	p.API.LogDebug("Post generated", "post", post)

	// Avoid possible duplication
	postInfo, _ := p.store.GetPostInfoByMSTeamsID(msg.ChatID+msg.ChannelID, msg.ID)
	if postInfo != nil {
		p.API.LogDebug("duplicated post")
		p.updateLastReceivedChangeDate(msg.LastUpdateAt)
		return
	}

	newPost, appErr := p.API.CreatePost(post)
	if appErr != nil {
		p.API.LogError("Unable to create post", "post", post, "error", appErr)
		return
	}

	p.API.LogDebug("Post created", "post", newPost)

	p.updateLastReceivedChangeDate(msg.LastUpdateAt)
	if newPost != nil && newPost.Id != "" && msg.ID != "" {
		err = p.store.LinkPosts(storemodels.PostInfo{MattermostID: newPost.Id, MSTeamsChannel: msg.ChatID + msg.ChannelID, MSTeamsID: msg.ID, MSTeamsLastUpdateAt: msg.LastUpdateAt})
		if err != nil {
			p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
		}
	}
}

func (p *Plugin) handleUpdatedActivity(activityIds msteams.ActivityIds) {
	msg, chat, err := p.getMessageAndChatFromActivityIds(activityIds)
	if err != nil {
		p.API.LogError("Unable to get original message", "error", err.Error())
		return
	}

	if msg == nil {
		p.API.LogDebug("Unable to get the message (probably because belongs to private chate in not-linked users)")
		return
	}

	if msg.UserID == "" {
		p.API.LogDebug("Skipping not user event", "msg", msg)
		return
	}

	msteamsUserID, _ := p.store.MattermostToTeamsUserID(p.userID)
	if msg.UserID == msteamsUserID {
		p.API.LogDebug("Skipping messages from bot user")
		p.updateLastReceivedChangeDate(msg.LastUpdateAt)
		return
	}

	postInfo, _ := p.store.GetPostInfoByMSTeamsID(msg.ChatID+msg.ChannelID, msg.ID)
	if postInfo == nil {
		p.updateLastReceivedChangeDate(msg.LastUpdateAt)
		return
	}

	// Ingnore if the change is already applied in the database
	if postInfo.MSTeamsLastUpdateAt.UnixMicro() == msg.LastUpdateAt.UnixMicro() {
		return
	}

	channelID := ""
	if chat == nil {
		var channelLink *storemodels.ChannelLink
		channelLink, err = p.store.GetLinkByMSTeamsChannelID(msg.TeamID, msg.ChannelID)
		if err != nil || channelLink == nil {
			p.API.LogError("Unable to find the subscription")
			return
		}
		channelID = channelLink.MattermostChannel
		if !p.configuration.SyncDirectMessages {
			// Skipping because direct/group messages are disabled
			return
		}
	} else {
		post, err := p.API.GetPost(postInfo.MattermostID)
		if err != nil {
			p.API.LogError("Unable to find the original post", "error", err.Error())
			return
		}
		channelID = post.ChannelId
	}

	senderID, err := p.store.TeamsToMattermostUserID(msg.UserID)
	if err != nil || senderID == "" {
		senderID = p.userID
	}

	post, err := p.msgToPost(channelID, msg, senderID)
	if err != nil {
		p.API.LogError("Unable to transform teams post in mattermost post", "message", msg, "error", err)
		return
	}

	post.Id = postInfo.MattermostID

	_, appErr := p.API.UpdatePost(post)
	if appErr != nil {
		p.API.LogError("Unable to update post", "post", post, "error", appErr)
		return
	}

	p.updateLastReceivedChangeDate(msg.LastUpdateAt)
	p.API.LogError("Message reactions", "reactions", msg.Reactions, "error", err)
	p.handleReactions(postInfo.MattermostID, channelID, msg.Reactions)
}

func (p *Plugin) handleReactions(postID string, channelID string, reactions []msteams.Reaction) {
	if len(reactions) == 0 {
		return
	}
	p.API.LogDebug("Handling reactions", "reactions", reactions)

	postReactions, appErr := p.API.GetReactions(postID)
	if appErr != nil {
		return
	}

	postReactionsByUserAndEmoji := map[string]bool{}
	for _, pr := range postReactions {
		postReactionsByUserAndEmoji[pr.UserId+pr.EmojiName] = true
	}

	allReactions := map[string]bool{}
	for _, reaction := range reactions {
		emojiName, ok := emojisReverseMap[reaction.Reaction]
		if !ok {
			p.API.LogError("Not code reaction found for reaction", "reaction", reaction.Reaction)
			continue
		}
		reactionUserID, err := p.store.TeamsToMattermostUserID(reaction.UserID)
		if err != nil {
			p.API.LogError("unable to find the user for the reaction", "reaction", reaction.Reaction)
			continue
		}
		allReactions[reactionUserID+emojiName] = true
	}

	for _, r := range postReactions {
		if !allReactions[r.UserId+r.EmojiName] {
			r.ChannelId = "removedfromplugin"
			appErr = p.API.RemoveReaction(r)
			if appErr != nil {
				p.API.LogError("Unable to remove reaction", "error", appErr.Error())
			}
		}
	}

	for _, reaction := range reactions {
		reactionUserID, err := p.store.TeamsToMattermostUserID(reaction.UserID)
		if err != nil {
			p.API.LogError("unable to find the user for the reaction", "reaction", reaction.Reaction)
			continue
		}
		emojiName, ok := emojisReverseMap[reaction.Reaction]
		if !ok {
			p.API.LogError("Not code reaction found for reaction", "reaction", reaction.Reaction)
			continue
		}
		if !postReactionsByUserAndEmoji[reactionUserID+emojiName] {
			r, appErr := p.API.AddReaction(&model.Reaction{
				UserId:    reactionUserID,
				PostId:    postID,
				ChannelId: channelID,
				EmojiName: emojiName,
			})
			if appErr != nil {
				p.API.LogError("failed to create the reaction", "err", appErr)
				continue
			}
			p.API.LogError("Added reaction", "reaction", r)
		}
	}
}

func (p *Plugin) handleDeletedActivity(activityIds msteams.ActivityIds) {
	postInfo, _ := p.store.GetPostInfoByMSTeamsID(activityIds.ChatID+activityIds.ChannelID, activityIds.MessageID)
	if postInfo == nil {
		return
	}

	appErr := p.API.DeletePost(postInfo.MattermostID)
	if appErr != nil {
		p.API.LogError("Unable to to delete post", "msgID", postInfo.MattermostID, "error", appErr)
		return
	}
}

func (p *Plugin) msgToPost(channelID string, msg *msteams.Message, senderID string) (*model.Post, error) {
	text := convertToMD(msg.Text)
	props := make(map[string]interface{})
	rootID := ""

	if msg.ReplyToID != "" {
		rootInfo, _ := p.store.GetPostInfoByMSTeamsID(msg.ChatID+msg.ChannelID, msg.ReplyToID)
		if rootInfo != nil {
			rootID = rootInfo.MattermostID
		}
	}

	newText, attachments, parentID := p.handleAttachments(channelID, text, msg)
	text = newText
	if parentID != "" {
		rootID = parentID
	}

	if rootID == "" && msg.Subject != "" {
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

func (p *Plugin) getChatChannelID(chat *msteams.Chat, msteamsUserID string) (string, error) {
	userIDs := []string{}
	for _, member := range chat.Members {
		mmUserID, err := p.store.TeamsToMattermostUserID(member.UserID)
		if err != nil || mmUserID == "" {
			u, appErr := p.API.GetUserByEmail(member.UserID + "@msteamssync")
			if appErr != nil {
				var appErr2 *model.AppError
				memberUUID := uuid.Parse(member.UserID)
				encoding := base32.NewEncoding("ybndrfg8ejkmcpqxot1uwisza345h769").WithPadding(base32.NoPadding)
				shortUserID := encoding.EncodeToString(memberUUID)
				u, appErr2 = p.API.CreateUser(&model.User{
					Username:  slug.Make(member.DisplayName) + "-" + member.UserID,
					FirstName: member.DisplayName,
					Email:     member.UserID + "@msteamssync",
					Password:  model.NewId(),
					RemoteId:  &shortUserID,
				})
				if appErr2 != nil {
					return "", appErr2
				}
			}
			err := p.store.SetUserInfo(u.Id, member.UserID, nil)
			if err != nil {
				p.API.LogError("Unable to link the new created mirror user", "error", err.Error())
			}
			mmUserID = u.Id
		}
		userIDs = append(userIDs, mmUserID)
	}
	if len(userIDs) < 2 {
		return "", errors.New("not enough user for creating a channel")
	}

	if chat.Type == "D" {
		channel, appErr := p.API.GetDirectChannel(userIDs[0], userIDs[1])
		if appErr != nil {
			return "", appErr
		}
		return channel.Id, nil
	}
	if chat.Type == "G" {
		channel, appErr := p.API.GetGroupChannel(userIDs)
		if appErr != nil {
			return "", appErr
		}
		return channel.Id, nil
	}
	return "", errors.New("dm/gm not found")
}

func (p *Plugin) updateLastReceivedChangeDate(t time.Time) {
	err := p.API.KVSet(lastReceivedChangeKey, []byte(strconv.FormatInt(t.UnixMicro(), 10)))
	if err != nil {
		p.API.LogError("Unable to store properly the last received change")
	}
}
