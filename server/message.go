package main

import (
	"bytes"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/enescakir/emoji"
	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/pkg/errors"
	"gitlab.com/golang-commonmark/markdown"
)

func (p *Plugin) SendChat(srcUser string, usersIDs []string, post *model.Post, containsRemoteUser bool) (string, error) {
	parentID := ""
	if post.RootId != "" {
		parentInfo, _ := p.store.GetPostInfoByMattermostID(post.RootId)
		if parentInfo != nil {
			parentID = parentInfo.MSTeamsID
		}
	}

	_, err := p.store.MattermostToTeamsUserID(srcUser)
	if err != nil {
		if containsRemoteUser {
			p.handlePromptForConnection(srcUser, post.ChannelId)
		}
		return "", err
	}

	client, err := p.GetClientForUser(srcUser)
	if err != nil {
		if containsRemoteUser {
			p.handlePromptForConnection(srcUser, post.ChannelId)
		}
		return "", err
	}

	teamsUsersIDs := make([]string, len(usersIDs))
	for idx, userID := range usersIDs {
		var teamsUserID string
		teamsUserID, err = p.store.MattermostToTeamsUserID(userID)
		if err != nil {
			p.API.LogWarn("Unable to get Teams user ID corresponding to MM user ID", "user_id", userID, "error", err)
			return "", err
		}
		teamsUsersIDs[idx] = teamsUserID
	}

	text := post.Message

	chat, err := client.CreateOrGetChatForUsers(teamsUsersIDs)
	if err != nil {
		p.API.LogWarn("Failed to create or get the chat", "error", err)
		return "", err
	}

	var attachments []*clientmodels.Attachment
	if p.GetSyncFileAttachments() {
		for _, fileID := range post.FileIds {
			fileInfo, appErr := p.API.GetFileInfo(fileID)
			if appErr != nil {
				p.API.LogWarn("Unable to get file info", "error", appErr)
				p.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, true)
				continue
			}
			fileData, appErr := p.API.GetFile(fileInfo.Id)
			if appErr != nil {
				p.API.LogWarn("Error in getting file attachment from Mattermost", "error", appErr)
				p.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, true)
				continue
			}

			fileName, fileExtension := getFileNameAndExtension(fileInfo.Name)
			var attachment *clientmodels.Attachment
			attachment, err = client.UploadFile("", "", fileName+"_"+fileInfo.Id+fileExtension, int(fileInfo.Size), fileInfo.MimeType, bytes.NewReader(fileData), chat)
			if err != nil {
				p.API.LogWarn("Error in uploading file attachment to MS Teams", "error", err)
				p.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToUploadFileOnTeams, true)
				continue
			}
			attachments = append(attachments, attachment)
			p.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMattermost, "", true)
		}
	} else if len(post.FileIds) > 0 {
		_, appErr := p.API.CreatePost(&model.Post{
			ChannelId: post.ChannelId,
			RootId:    post.RootId,
			UserId:    p.GetBotUserID(),
			Message:   "Attachments sent from Mattermost aren't yet delivered to Microsoft Teams.",
		})
		if appErr != nil {
			p.API.LogWarn("Failed to notify channel of skipped attachment", "channel_id", post.ChannelId, "post_id", post.Id, "error", appErr)
		}
	}

	md := markdown.New(markdown.XHTMLOutput(true), markdown.Typographer(false))
	content := md.RenderToString([]byte(emoji.Parse(text)))

	content, mentions := p.getMentionsData(content, "", "", chat.ID, client)

	var parentMessage *clientmodels.Message
	if parentID != "" {
		parentMessage, err = client.GetChatMessage(chat.ID, parentID)
		if err != nil {
			p.API.LogWarn("Error in getting parent chat message", "error", err)
		}
	}

	content += p.MessageFingerprint()

	newMessage, err := client.SendChat(chat.ID, content, parentMessage, attachments, mentions)
	if err != nil {
		p.API.LogWarn("Error creating post on MS Teams", "error", err.Error())
		return "", err
	}

	p.GetMetrics().ObserveMessage(metrics.ActionCreated, metrics.ActionSourceMattermost, true)
	p.GetMetrics().ObserveMessageDelay(metrics.ActionCreated, metrics.ActionSourceMattermost, true, newMessage.CreateAt.Sub(time.UnixMilli(post.CreateAt)))

	if post.Id != "" {
		if err := p.store.LinkPosts(storemodels.PostInfo{MattermostID: post.Id, MSTeamsChannel: chat.ID, MSTeamsID: newMessage.ID, MSTeamsLastUpdateAt: newMessage.LastUpdateAt}); err != nil {
			p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
		}
	}

	if err := p.store.SetUserLastChatSentAt(post.UserId, storemodels.MilliToMicroSeconds(post.CreateAt)); err != nil {
		p.API.LogWarn("Unable to set user last chat sent At", "user_id", post.UserId, "post_id", post.Id, "error", err.Error())
	}

	return newMessage.ID, nil
}

func (p *Plugin) Send(teamID, channelID string, user *model.User, post *model.Post) (string, error) {
	parentID := ""
	if post.RootId != "" {
		parentInfo, _ := p.store.GetPostInfoByMattermostID(post.RootId)
		if parentInfo != nil {
			parentID = parentInfo.MSTeamsID
		}
	}

	text := post.Message
	client, err := p.GetClientForUser(user.Id)
	if err != nil {
		client, err = p.GetClientForUser(p.botUserID)
		if err != nil {
			return "", err
		}
		text = user.Username + ":\n\n" + post.Message
	}

	var attachments []*clientmodels.Attachment

	if p.GetSyncFileAttachments() {
		for _, fileID := range post.FileIds {
			fileInfo, appErr := p.API.GetFileInfo(fileID)
			if appErr != nil {
				p.API.LogWarn("Unable to get file info", "error", appErr)
				p.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, false)
				continue
			}
			fileData, appErr := p.API.GetFile(fileInfo.Id)
			if appErr != nil {
				p.API.LogWarn("Error in getting file attachment from Mattermost", "error", appErr)
				p.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, false)
				continue
			}

			fileName, fileExtension := getFileNameAndExtension(fileInfo.Name)
			var attachment *clientmodels.Attachment
			attachment, err = client.UploadFile(teamID, channelID, fileName+"_"+fileInfo.Id+fileExtension, int(fileInfo.Size), fileInfo.MimeType, bytes.NewReader(fileData), nil)
			if err != nil {
				p.API.LogWarn("Error in uploading file attachment to MS Teams", "error", err)
				p.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToUploadFileOnTeams, false)
				continue
			}
			attachments = append(attachments, attachment)
			p.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMattermost, "", false)
		}
	} else if len(post.FileIds) > 0 {
		_, appErr := p.API.CreatePost(&model.Post{
			ChannelId: post.ChannelId,
			RootId:    post.RootId,
			UserId:    p.GetBotUserID(),
			Message:   "Attachments sent from Mattermost aren't yet delivered to Microsoft Teams.",
		})
		if appErr != nil {
			p.API.LogWarn("Failed to notify channel of skipped attachment", "channel_id", channelID, "post_id", post.Id, "error", appErr)
		}
	}

	md := markdown.New(markdown.XHTMLOutput(true), markdown.Typographer(false))
	content := md.RenderToString([]byte(emoji.Parse(text)))

	content, mentions := p.getMentionsData(content, teamID, channelID, "", client)

	content += p.MessageFingerprint()

	newMessage, err := client.SendMessageWithAttachments(teamID, channelID, parentID, content, attachments, mentions)
	if err != nil {
		p.API.LogWarn("Error creating post on MS Teams", "error", err.Error())
		return "", err
	}

	p.GetMetrics().ObserveMessage(metrics.ActionCreated, metrics.ActionSourceMattermost, false)
	p.GetMetrics().ObserveMessageDelay(metrics.ActionCreated, metrics.ActionSourceMattermost, false, newMessage.CreateAt.Sub(time.UnixMilli(post.CreateAt)))
	if post.Id != "" {
		if err := p.store.LinkPosts(storemodels.PostInfo{MattermostID: post.Id, MSTeamsChannel: channelID, MSTeamsID: newMessage.ID, MSTeamsLastUpdateAt: newMessage.LastUpdateAt}); err != nil {
			p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
		}
	}
	return newMessage.ID, nil
}

func (p *Plugin) Delete(teamID, channelID string, user *model.User, post *model.Post) error {
	parentID := ""
	if post.RootId != "" {
		parentInfo, _ := p.store.GetPostInfoByMattermostID(post.RootId)
		if parentInfo != nil {
			parentID = parentInfo.MSTeamsID
		}
	}

	client, err := p.GetClientForUser(user.Id)
	if err != nil {
		client, err = p.GetClientForUser(p.botUserID)
		if err != nil {
			return err
		}
	}

	postInfo, err := p.store.GetPostInfoByMattermostID(post.Id)
	if err != nil {
		p.API.LogWarn("Error getting post info", "error", err)
		return err
	}

	if postInfo == nil {
		p.API.LogWarn("Error deleting post, post not found.")
		return errors.New("post not found")
	}

	if err := client.DeleteMessage(teamID, channelID, parentID, postInfo.MSTeamsID); err != nil {
		p.API.LogWarn("Error deleting post from MS Teams", "error", err)
		return err
	}

	p.GetMetrics().ObserveMessage(metrics.ActionDeleted, metrics.ActionSourceMattermost, false)
	return nil
}

func (p *Plugin) DeleteChat(post *model.Post) error {
	client, err := p.GetClientForUser(post.UserId)
	if err != nil {
		return err
	}

	chatID, err := p.GetChatIDForChannel(client, post.ChannelId)
	if err != nil {
		return err
	}

	postInfo, err := p.store.GetPostInfoByMattermostID(post.Id)
	if err != nil {
		p.API.LogWarn("Error getting post info", "error", err)
		return err
	}

	if postInfo == nil {
		p.API.LogWarn("Error deleting post, post not found.")
		return errors.New("post not found")
	}

	if err := client.DeleteChatMessage(post.UserId, chatID, postInfo.MSTeamsID); err != nil {
		p.API.LogWarn("Error deleting post from MS Teams", "error", err)
		return err
	}

	p.GetMetrics().ObserveMessage(metrics.ActionDeleted, metrics.ActionSourceMattermost, true)
	return nil
}

func (p *Plugin) Update(teamID, channelID string, user *model.User, newPost *model.Post, updateRequired bool) error {
	parentID := ""
	if newPost.RootId != "" {
		parentInfo, _ := p.store.GetPostInfoByMattermostID(newPost.RootId)
		if parentInfo != nil {
			parentID = parentInfo.MSTeamsID
		}
	}

	text := newPost.Message

	client, err := p.GetClientForUser(user.Id)
	if err != nil {
		client, err = p.GetClientForUser(p.botUserID)
		if err != nil {
			return err
		}
		text = user.Username + ":\n\n" + newPost.Message
	}

	postInfo, err := p.store.GetPostInfoByMattermostID(newPost.Id)
	if err != nil {
		p.API.LogWarn("Error getting post info", "error", err)
		return err
	}
	if postInfo == nil {
		p.API.LogWarn("Error updating post, post not found.")
		return errors.New("post not found")
	}

	mutex, err := cluster.NewMutex(p.API, "post_mutex_"+newPost.Id)
	if err != nil {
		return err
	}
	mutex.Lock()
	defer mutex.Unlock()

	var updatedMessage *clientmodels.Message
	if updateRequired {
		// TODO: Add the logic of processing the attachments and uploading new files to Teams
		// once Mattermost comes up with the feature of editing attachments
		md := markdown.New(markdown.XHTMLOutput(true), markdown.Typographer(false), markdown.LangPrefix("CodeMirror language-"))
		content := md.RenderToString([]byte(emoji.Parse(text)))
		content, mentions := p.getMentionsData(content, teamID, channelID, "", client)
		updatedMessage, err = client.UpdateMessage(teamID, channelID, parentID, postInfo.MSTeamsID, content, mentions)
		if err != nil {
			p.API.LogWarn("Error updating the post on MS Teams", "error", err)
			// If the error is regarding payment required for metered APIs, ignore it and continue because
			// the post is updated regardless
			if !strings.Contains(err.Error(), "code: PaymentRequired") {
				return err
			}
		}

		p.GetMetrics().ObserveMessage(metrics.ActionUpdated, metrics.ActionSourceMattermost, false)
	} else {
		updatedMessage, err = getUpdatedMessage(teamID, channelID, parentID, postInfo.MSTeamsID, client)
		if err != nil {
			p.API.LogWarn("Error in getting the message from MS Teams", "error", err)
			return err
		}
	}

	if err = p.store.LinkPosts(storemodels.PostInfo{MattermostID: newPost.Id, MSTeamsChannel: channelID, MSTeamsID: postInfo.MSTeamsID, MSTeamsLastUpdateAt: updatedMessage.LastUpdateAt}); err != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
	}

	return nil
}

func (p *Plugin) UpdateChat(chatID string, user *model.User, newPost *model.Post, updateRequired bool) error {
	postInfo, err := p.store.GetPostInfoByMattermostID(newPost.Id)
	if err != nil {
		p.API.LogWarn("Error getting post info", "error", err)
		return err
	}
	if postInfo == nil {
		p.API.LogWarn("Error updating post, post not found.")
		return errors.New("post not found")
	}

	text := newPost.Message

	client, err := p.GetClientForUser(user.Id)
	if err != nil {
		return err
	}

	var updatedMessage *clientmodels.Message

	if updateRequired {
		md := markdown.New(markdown.XHTMLOutput(true), markdown.Typographer(false), markdown.LangPrefix("CodeMirror language-"))
		content := md.RenderToString([]byte(emoji.Parse(text)))
		content, mentions := p.getMentionsData(content, "", "", chatID, client)
		updatedMessage, err = client.UpdateChatMessage(chatID, postInfo.MSTeamsID, content, mentions)
		if err != nil {
			p.API.LogWarn("Error updating the post on MS Teams", "error", err)
			// If the error is regarding payment required for metered APIs, ignore it and continue because
			// the post is updated regardless
			if !strings.Contains(err.Error(), "code: PaymentRequired") {
				return err
			}
		}

		p.GetMetrics().ObserveMessage(metrics.ActionUpdated, metrics.ActionSourceMattermost, true)
	} else {
		updatedMessage, err = client.GetChatMessage(chatID, postInfo.MSTeamsID)
		if err != nil {
			p.API.LogWarn("Error getting the updated message from MS Teams", "error", err)
			return err
		}
	}

	if err = p.store.LinkPosts(storemodels.PostInfo{MattermostID: newPost.Id, MSTeamsChannel: chatID, MSTeamsID: postInfo.MSTeamsID, MSTeamsLastUpdateAt: updatedMessage.LastUpdateAt}); err != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
	}

	return nil
}

func (p *Plugin) GetChatIDForChannel(client msteams.Client, channelID string) (string, error) {
	channel, appErr := p.API.GetChannel(channelID)
	if appErr != nil {
		p.API.LogWarn("Unable to get MM channel", "channel_id", channelID, "error", appErr.DetailedError)
		return "", appErr
	}
	if !channel.IsGroupOrDirect() {
		return "", errors.New("invalid channel type, chatID is only available for direct messages and group messages")
	}

	members, appErr := p.API.GetChannelMembers(channelID, 0, math.MaxInt32)
	if appErr != nil {
		p.API.LogWarn("Unable to get MM channel members", "channel_id", channelID, "error", appErr.DetailedError)
		return "", appErr
	}

	teamsUsersIDs := make([]string, len(members))
	for idx, m := range members {
		teamsUserID, err := p.store.MattermostToTeamsUserID(m.UserId)
		if err != nil {
			return "", err
		}
		teamsUsersIDs[idx] = teamsUserID
	}

	chat, err := client.CreateOrGetChatForUsers(teamsUsersIDs)
	if err != nil {
		p.API.LogWarn("Unable to create or get chat for users", "error", err.Error())
		return "", err
	}

	return chat.ID, nil
}

func (p *Plugin) getMentionsData(message, teamID, channelID, chatID string, client msteams.Client) (string, []models.ChatMessageMentionable) {
	specialMentions := map[string]bool{
		"all":     true,
		"channel": true,
		"here":    true,
	}

	re := regexp.MustCompile(`@([a-z0-9.\-_]+)`)
	channelMentions := re.FindAllString(message, -1)

	mentions := []models.ChatMessageMentionable{}

	for id, m := range channelMentions {
		username := m[1:]
		mentionedText := m

		mention := models.NewChatMessageMention()
		mentionedID := int32(id)
		mention.SetId(&mentionedID)

		mentioned := models.NewChatMessageMentionedIdentitySet()
		conversation := models.NewTeamworkConversationIdentity()

		if specialMentions[username] {
			if chatID != "" {
				chat, err := client.GetChat(chatID)
				if err != nil {
					p.API.LogWarn("Unable to get MS Teams chat", "error", err.Error())
				} else {
					if chat.Type == "G" {
						mentionedText = "Everyone"
					} else {
						continue
					}
				}

				conversation.SetId(&chatID)
			} else {
				msChannel, err := client.GetChannelInTeam(teamID, channelID)
				if err != nil {
					p.API.LogWarn("Unable to get MS Teams channel", "error", err.Error())
				} else {
					mentionedText = msChannel.DisplayName
				}

				conversation.SetId(&channelID)
			}

			conversation.SetDisplayName(&mentionedText)

			conversationIdentityType := models.CHANNEL_TEAMWORKCONVERSATIONIDENTITYTYPE
			conversation.SetConversationIdentityType(&conversationIdentityType)
			mentioned.SetConversation(conversation)
		} else {
			mmUser, err := p.API.GetUserByUsername(username)
			if err != nil {
				p.API.LogWarn("Unable to get user by username", "error", err.Error())
				continue
			}

			msteamsUserID, getErr := p.store.MattermostToTeamsUserID(mmUser.Id)
			if getErr != nil {
				p.API.LogWarn("Unable to get MS Teams user ID", "error", getErr.Error())
				continue
			}

			msteamsUser, getErr := client.GetUser(msteamsUserID)
			if getErr != nil {
				p.API.LogWarn("Unable to get MS Teams user", "teams_user_id", msteamsUserID, "error", getErr.Error())
				continue
			}

			mentionedText = msteamsUser.DisplayName

			identity := models.NewIdentity()
			identity.SetId(&msteamsUserID)
			identity.SetDisplayName(&msteamsUser.DisplayName)

			additionalData := map[string]interface{}{
				"userIdentityType": "aadUser",
			}

			identity.SetAdditionalData(additionalData)
			mentioned.SetUser(identity)
		}

		message = strings.Replace(message, m, fmt.Sprintf("<at id=\"%s\">%s</at>", fmt.Sprint(id), mentionedText), 1)
		mention.SetMentionText(&mentionedText)
		mention.SetMentioned(mentioned)

		mentions = append(mentions, mention)
	}

	return message, mentions
}

func getFileNameAndExtension(path string) (string, string) {
	fileExtension := filepath.Ext(path)
	fileName := strings.TrimSuffix(path, fileExtension)
	return fileName, fileExtension
}

func getUpdatedMessage(teamID, channelID, parentID, msteamsID string, client msteams.Client) (*clientmodels.Message, error) {
	if parentID != "" {
		return client.GetReply(teamID, channelID, parentID, msteamsID)
	}

	return client.GetMessage(teamID, channelID, msteamsID)
}

func (p *Plugin) ShouldSyncDMGMChannel(channel *model.Channel) (bool, string) {
	switch channel.Type {
	case model.ChannelTypeDirect:
		if !p.GetSyncDirectMessages() {
			return false, metrics.DiscardedReasonDirectMessagesDisabled
		}
	case model.ChannelTypeGroup:
		if !p.GetSyncGroupMessages() {
			return false, metrics.DiscardedReasonGroupMessagesDisabled
		}
	default:
		return false, metrics.DiscardedReasonOther
	}

	return true, metrics.DiscardedReasonNone
}
