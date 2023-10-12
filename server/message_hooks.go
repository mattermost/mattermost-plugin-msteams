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
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/pkg/errors"
	"gitlab.com/golang-commonmark/markdown"
)

func (p *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	if post.Props != nil {
		if _, ok := post.Props["msteams_sync_"+p.userID].(bool); ok {
			return
		}
	}

	if post.IsSystemMessage() {
		p.API.LogDebug("Skipping system message post", "PostID", post.Id)
		return
	}

	link, err := p.store.GetLinkByChannelID(post.ChannelId)
	if err != nil || link == nil {
		channel, appErr := p.API.GetChannel(post.ChannelId)
		if appErr != nil {
			return
		}
		if (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) && p.getConfiguration().SyncDirectMessages {
			members, appErr := p.API.GetChannelMembers(post.ChannelId, 0, math.MaxInt32)
			if appErr != nil {
				return
			}
			dstUsers := []string{}
			for _, m := range members {
				dstUsers = append(dstUsers, m.UserId)
			}
			_, err = p.SendChat(post.UserId, dstUsers, post)
			if err != nil {
				p.API.LogWarn("Unable to handle message sent", "error", err.Error())
			}
		}
		return
	}

	user, _ := p.API.GetUser(post.UserId)

	_, err = p.Send(link.MSTeamsTeam, link.MSTeamsChannel, user, post)
	if err != nil {
		p.API.LogWarn("Unable to handle message sent", "error", err.Error())
	}
}

func (p *Plugin) ReactionHasBeenAdded(c *plugin.Context, reaction *model.Reaction) {
	updateRequired := true
	if c.RequestId == "" {
		_, ignoreHookForReaction := p.activityHandler.IgnorePluginHooksMap.LoadAndDelete(fmt.Sprintf("%s_%s_%s", reaction.PostId, reaction.UserId, reaction.EmojiName))
		updateRequired = !ignoreHookForReaction
	}

	p.API.LogDebug("Reaction added hook", "reaction", reaction)
	postInfo, err := p.store.GetPostInfoByMattermostID(reaction.PostId)
	if err != nil || postInfo == nil {
		p.API.LogDebug("Unable to find Teams post corresponding to MM post", "mmPostID", reaction.PostId)
		return
	}

	link, err := p.store.GetLinkByChannelID(reaction.ChannelId)
	if err != nil || link == nil {
		channel, appErr := p.API.GetChannel(reaction.ChannelId)
		if appErr != nil {
			return
		}
		if (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) && p.getConfiguration().SyncDirectMessages {
			err = p.SetChatReaction(postInfo.MSTeamsID, reaction.UserId, reaction.ChannelId, reaction.EmojiName, updateRequired)
			if err != nil {
				p.API.LogWarn("Unable to handle message reaction set", "error", err.Error())
			}
		}
		return
	}

	post, appErr := p.API.GetPost(reaction.PostId)
	if appErr != nil {
		p.API.LogError("Unable to get the post from the reaction", "reaction", reaction, "error", appErr)
		return
	}

	if err = p.SetReaction(link.MSTeamsTeam, link.MSTeamsChannel, reaction.UserId, post, reaction.EmojiName, updateRequired); err != nil {
		p.API.LogWarn("Unable to handle message reaction set", "error", err.Error())
	}
}

func (p *Plugin) ReactionHasBeenRemoved(_ *plugin.Context, reaction *model.Reaction) {
	p.API.LogDebug("Removing reaction hook", "reaction", reaction)
	if reaction.ChannelId == "removedfromplugin" {
		p.API.LogInfo("Ignore reaction that has been triggered from the plugin handler")
		return
	}
	postInfo, err := p.store.GetPostInfoByMattermostID(reaction.PostId)
	if err != nil || postInfo == nil {
		p.API.LogDebug("Unable to find Teams post corresponding to MM post", "mmPostID", reaction.PostId)
		return
	}

	post, appErr := p.API.GetPost(reaction.PostId)
	if appErr != nil {
		p.API.LogError("Unable to get the post from the reaction", "reaction", reaction, "error", appErr.DetailedError)
		return
	}

	link, err := p.store.GetLinkByChannelID(post.ChannelId)
	if err != nil || link == nil {
		channel, appErr := p.API.GetChannel(post.ChannelId)
		if appErr != nil {
			return
		}
		if (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) && p.getConfiguration().SyncDirectMessages {
			err = p.UnsetChatReaction(postInfo.MSTeamsID, reaction.UserId, post.ChannelId, reaction.EmojiName)
			if err != nil {
				p.API.LogWarn("Unable to handle chat message reaction unset", "error", err.Error())
			}
		}
		return
	}

	err = p.UnsetReaction(link.MSTeamsTeam, link.MSTeamsChannel, reaction.UserId, post, reaction.EmojiName)
	if err != nil {
		p.API.LogWarn("Unable to handle message reaction unset", "error", err.Error())
	}
}

func (p *Plugin) MessageHasBeenUpdated(c *plugin.Context, newPost, oldPost *model.Post) {
	updateRequired := true
	if c.RequestId == "" {
		_, ignoreHook := p.activityHandler.IgnorePluginHooksMap.LoadAndDelete(fmt.Sprintf("post_%s", newPost.Id))
		updateRequired = !ignoreHook
	}

	client, err := p.GetClientForUser(newPost.UserId)
	if err != nil {
		return
	}

	user, _ := p.API.GetUser(newPost.UserId)

	link, err := p.store.GetLinkByChannelID(newPost.ChannelId)
	if err != nil || link == nil {
		channel, appErr := p.API.GetChannel(newPost.ChannelId)
		if appErr != nil {
			return
		}
		if channel.Type != model.ChannelTypeGroup && channel.Type != model.ChannelTypeDirect {
			return
		}
		if !p.getConfiguration().SyncDirectMessages {
			return
		}

		members, appErr := p.API.GetChannelMembers(newPost.ChannelId, 0, math.MaxInt32)
		if appErr != nil {
			return
		}
		usersIDs := []string{}
		for _, m := range members {
			var teamsUserID string
			teamsUserID, err = p.store.MattermostToTeamsUserID(m.UserId)
			if err != nil {
				return
			}
			usersIDs = append(usersIDs, teamsUserID)
		}
		var chat *msteams.Chat
		chat, err = client.CreateOrGetChatForUsers(usersIDs)
		if err != nil {
			p.API.LogError("Unable to create or get chat for users", "error", err.Error())
			return
		}
		err = p.UpdateChat(chat.ID, user, newPost, oldPost, updateRequired)
		if err != nil {
			p.API.LogError("Unable to handle message update", "error", err.Error())
		}
		return
	}

	err = p.Update(link.MSTeamsTeam, link.MSTeamsChannel, user, newPost, oldPost, updateRequired)
	if err != nil {
		p.API.LogError("Unable to handle message update", "error", err.Error())
	}
}

func (p *Plugin) SetChatReaction(teamsMessageID, srcUser, channelID, emojiName string, updateRequired bool) error {
	p.API.LogDebug("Setting chat reaction", "srcUser", srcUser, "emojiName", emojiName, "channelID", channelID)

	srcUserID, err := p.store.MattermostToTeamsUserID(srcUser)
	if err != nil {
		p.handlePromptForConnection(srcUser, channelID)
		return err
	}

	client, err := p.GetClientForUser(srcUser)
	if err != nil {
		p.handlePromptForConnection(srcUser, channelID)
		return err
	}

	chatID, err := p.GetChatIDForChannel(client, channelID)
	if err != nil {
		return err
	}

	var teamsMessage *msteams.Message
	tx, err := p.store.BeginTx()
	if err != nil {
		return err
	}

	var txErr error
	defer func() {
		if txErr != nil {
			if err := p.store.RollbackTx(tx); err != nil {
				p.API.LogWarn("Unable to rollback database transaction", "error", err.Error())
			}
			return
		}

		if err := p.store.CommitTx(tx); err != nil {
			p.API.LogWarn("Unable to commit database transaction", "error", err.Error())
		}
	}()

	if txErr = p.store.LockPostByMSTeamsPostID(tx, teamsMessageID); txErr != nil {
		return txErr
	}

	if updateRequired {
		teamsMessage, txErr = client.SetChatReaction(chatID, teamsMessageID, srcUserID, emoji.Parse(":"+emojiName+":"))
		if txErr != nil {
			p.API.LogError("Error creating post reaction", "error", txErr.Error())
			return txErr
		}
	} else {
		teamsMessage, txErr = client.GetChatMessage(chatID, teamsMessageID)
		if txErr != nil {
			p.API.LogWarn("Error getting the msteams post metadata", "error", txErr.Error())
			return txErr
		}
	}

	if txErr = p.store.SetPostLastUpdateAtByMSTeamsID(teamsMessageID, teamsMessage.LastUpdateAt, tx); txErr != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", txErr.Error())
	}

	return nil
}

func (p *Plugin) SetReaction(teamID, channelID, userID string, post *model.Post, emojiName string, updateRequired bool) error {
	p.API.LogDebug("Setting reaction", "teamID", teamID, "channelID", channelID, "PostID", post.Id, "emojiName", emojiName)

	postInfo, err := p.store.GetPostInfoByMattermostID(post.Id)
	if err != nil {
		return err
	}

	if postInfo == nil {
		return errors.New("teams message not found")
	}

	parentID := ""
	if post.RootId != "" {
		parentInfo, _ := p.store.GetPostInfoByMattermostID(post.RootId)
		if parentInfo != nil {
			parentID = parentInfo.MSTeamsID
		}
	}

	client, err := p.GetClientForUser(userID)
	if err != nil {
		return err
	}

	var teamsMessage *msteams.Message
	tx, err := p.store.BeginTx()
	if err != nil {
		return err
	}

	var txErr error
	defer func() {
		if txErr != nil {
			if err := p.store.RollbackTx(tx); err != nil {
				p.API.LogWarn("Unable to rollback database transaction", "error", err.Error())
			}
			return
		}

		if err := p.store.CommitTx(tx); err != nil {
			p.API.LogWarn("Unable to commit database transaction", "error", err.Error())
		}
	}()

	if txErr = p.store.LockPostByMMPostID(tx, postInfo.MattermostID); txErr != nil {
		return txErr
	}

	if updateRequired {
		teamsUserID, _ := p.store.MattermostToTeamsUserID(userID)
		teamsMessage, txErr = client.SetReaction(teamID, channelID, parentID, postInfo.MSTeamsID, teamsUserID, emoji.Parse(":"+emojiName+":"))
		if txErr != nil {
			p.API.LogError("Error setting reaction", "error", txErr.Error())
			return txErr
		}
	} else {
		teamsMessage, txErr = getUpdatedMessage(teamID, channelID, parentID, postInfo.MSTeamsID, client)
		if txErr != nil {
			p.API.LogWarn("Error getting the msteams post metadata", "error", txErr.Error())
			return txErr
		}
	}

	if txErr = p.store.SetPostLastUpdateAtByMattermostID(postInfo.MattermostID, teamsMessage.LastUpdateAt, tx); txErr != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", txErr.Error())
	}

	return nil
}

func (p *Plugin) UnsetChatReaction(teamsMessageID, srcUser, channelID string, emojiName string) error {
	p.API.LogDebug("Unsetting chat reaction", "srcUser", srcUser, "emojiName", emojiName, "channelID", channelID)

	srcUserID, err := p.store.MattermostToTeamsUserID(srcUser)
	if err != nil {
		p.handlePromptForConnection(srcUser, channelID)
		return err
	}

	client, err := p.GetClientForUser(srcUser)
	if err != nil {
		p.handlePromptForConnection(srcUser, channelID)
		return err
	}

	chatID, err := p.GetChatIDForChannel(client, channelID)
	if err != nil {
		return err
	}

	tx, err := p.store.BeginTx()
	if err != nil {
		return err
	}

	var txErr error
	defer func() {
		if txErr != nil {
			if err := p.store.RollbackTx(tx); err != nil {
				p.API.LogWarn("Unable to rollback database transaction", "error", err.Error())
			}
			return
		}

		if err := p.store.CommitTx(tx); err != nil {
			p.API.LogWarn("Unable to commit database transaction", "error", err.Error())
		}
	}()

	if txErr = p.store.LockPostByMSTeamsPostID(tx, teamsMessageID); txErr != nil {
		return txErr
	}

	teamsMessage, txErr := client.UnsetChatReaction(chatID, teamsMessageID, srcUserID, emoji.Parse(":"+emojiName+":"))
	if txErr != nil {
		p.API.LogError("Error in removing the chat reaction", "emojiName", emojiName, "error", txErr.Error())
		return txErr
	}

	if txErr = p.store.SetPostLastUpdateAtByMSTeamsID(teamsMessageID, teamsMessage.LastUpdateAt, tx); txErr != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", txErr.Error())
	}

	return nil
}

func (p *Plugin) UnsetReaction(teamID, channelID, userID string, post *model.Post, emojiName string) error {
	p.API.LogDebug("Unsetting reaction", "teamID", teamID, "channelID", channelID, "PostID", post.Id, "emojiName", emojiName)

	postInfo, err := p.store.GetPostInfoByMattermostID(post.Id)
	if err != nil {
		return err
	}

	if postInfo == nil {
		return errors.New("teams message not found")
	}

	parentID := ""
	if post.RootId != "" {
		parentInfo, _ := p.store.GetPostInfoByMattermostID(post.RootId)
		if parentInfo != nil {
			parentID = parentInfo.MSTeamsID
		}
	}

	client, err := p.GetClientForUser(userID)
	if err != nil {
		return err
	}

	teamsUserID, _ := p.store.MattermostToTeamsUserID(userID)
	tx, err := p.store.BeginTx()
	if err != nil {
		return err
	}

	var txErr error
	defer func() {
		if txErr != nil {
			if err := p.store.RollbackTx(tx); err != nil {
				p.API.LogWarn("Unable to rollback database transaction", "error", err.Error())
			}
			return
		}

		if err := p.store.CommitTx(tx); err != nil {
			p.API.LogWarn("Unable to commit database transaction", "error", err.Error())
		}
	}()

	if txErr = p.store.LockPostByMMPostID(tx, postInfo.MattermostID); txErr != nil {
		return txErr
	}

	teamsMessage, txErr := client.UnsetReaction(teamID, channelID, parentID, postInfo.MSTeamsID, teamsUserID, emoji.Parse(":"+emojiName+":"))
	if txErr != nil {
		p.API.LogError("Error in removing the reaction", "emojiName", emojiName, "error", txErr.Error())
		return txErr
	}

	if txErr = p.store.SetPostLastUpdateAtByMattermostID(postInfo.MattermostID, teamsMessage.LastUpdateAt, tx); txErr != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", txErr.Error())
	}

	return nil
}

func (p *Plugin) SendChat(srcUser string, usersIDs []string, post *model.Post) (string, error) {
	p.API.LogDebug("Sending direct message to MS Teams", "SrcUser", srcUser, "UsersIDs", usersIDs, "PostID", post.Id)

	parentID := ""
	if post.RootId != "" {
		parentInfo, _ := p.store.GetPostInfoByMattermostID(post.RootId)
		if parentInfo != nil {
			parentID = parentInfo.MSTeamsID
		}
	}

	srcUserID, err := p.store.MattermostToTeamsUserID(srcUser)
	if err != nil {
		p.handlePromptForConnection(srcUser, post.ChannelId)
		return "", err
	}

	client, err := p.GetClientForUser(srcUser)
	if err != nil {
		p.handlePromptForConnection(srcUser, post.ChannelId)
		return "", err
	}

	teamsUsersIDs := make([]string, len(usersIDs))
	for idx, userID := range usersIDs {
		var teamsUserID string
		teamsUserID, err = p.store.MattermostToTeamsUserID(userID)
		if err != nil {
			p.API.LogDebug("Unable to get Teams user ID corresponding to MM user ID", "mmUserID", userID)
			return "", err
		}
		teamsUsersIDs[idx] = teamsUserID
	}

	p.API.LogDebug("Sending direct message to MS Teams", "SrcUserID", srcUserID, "TeamsUsersIDs", teamsUsersIDs, "PostID", post.Id)
	text := post.Message

	chat, err := client.CreateOrGetChatForUsers(teamsUsersIDs)
	if err != nil {
		p.API.LogError("Failed to create or get the chat", "error", err)
		return "", err
	}

	var attachments []*msteams.Attachment
	for _, fileID := range post.FileIds {
		fileInfo, appErr := p.API.GetFileInfo(fileID)
		if appErr != nil {
			p.API.LogWarn("Unable to get file info", "error", appErr)
			continue
		}
		fileData, appErr := p.API.GetFile(fileInfo.Id)
		if appErr != nil {
			p.API.LogWarn("Error in getting file attachment from Mattermost", "error", appErr)
			continue
		}

		fileName, fileExtension := getFileNameAndExtension(fileInfo.Name)
		var attachment *msteams.Attachment
		attachment, err = client.UploadFile("", "", fileName+"_"+fileInfo.Id+fileExtension, int(fileInfo.Size), fileInfo.MimeType, bytes.NewReader(fileData), chat)
		if err != nil {
			p.API.LogWarn("Error in uploading file attachment to MS Teams", "error", err)
			continue
		}
		attachments = append(attachments, attachment)
	}

	md := markdown.New(markdown.XHTMLOutput(true), markdown.Typographer(false))
	content := md.RenderToString([]byte(emoji.Parse(text)))

	content, mentions := p.getMentionsData(content, "", "", chat.ID, client)

	var parentMessage *msteams.Message
	if parentID != "" {
		parentMessage, err = client.GetChatMessage(chat.ID, parentID)
		if err != nil {
			p.API.LogWarn("Error in getting parent chat message", "error", err)
		}
	}

	newMessage, err := client.SendChat(chat.ID, content, parentMessage, attachments, mentions)
	if err != nil {
		p.API.LogError("Error creating post on MS Teams", "error", err.Error())
		return "", err
	}

	if post.Id != "" && newMessage != nil {
		if err := p.store.LinkPosts(storemodels.PostInfo{MattermostID: post.Id, MSTeamsChannel: chat.ID, MSTeamsID: newMessage.ID, MSTeamsLastUpdateAt: newMessage.LastUpdateAt}, nil); err != nil {
			p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
		}
	}
	return newMessage.ID, nil
}

func (p *Plugin) handlePromptForConnection(userID, channelID string) {
	promptInterval := p.getConfiguration().PromptIntervalForDMsAndGMs
	if promptInterval <= 0 {
		return
	}

	timestamp, err := p.store.GetDMAndGMChannelPromptTime(channelID, userID)
	if err != nil {
		p.API.LogDebug("Unable to get the last prompt timestamp for the channel", "ChannelID", channelID, "Error", err.Error())
	}

	if time.Until(timestamp) < -time.Hour*time.Duration(promptInterval) {
		p.sendBotEphemeralPost(userID, channelID, "Your Mattermost account is not connected to MS Teams so your activity will not be relayed to users on MS Teams. You can connect your account using the `/msteams-sync connect` slash command.")

		if err = p.store.StoreDMAndGMChannelPromptTime(channelID, userID, time.Now()); err != nil {
			p.API.LogDebug("Unable to store the last prompt timestamp for the channel", "ChannelID", channelID, "Error", err.Error())
		}
	}
}

func (p *Plugin) Send(teamID, channelID string, user *model.User, post *model.Post) (string, error) {
	p.API.LogDebug("Sending message to MS Teams", "TeamID", teamID, "ChannelID", channelID, "PostID", post.Id)

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
		client, err = p.GetClientForUser(p.userID)
		if err != nil {
			return "", err
		}
		text = user.Username + ":\n\n" + post.Message
	}

	var attachments []*msteams.Attachment
	for _, fileID := range post.FileIds {
		fileInfo, appErr := p.API.GetFileInfo(fileID)
		if appErr != nil {
			p.API.LogWarn("Unable to get file info", "error", appErr)
			continue
		}
		fileData, appErr := p.API.GetFile(fileInfo.Id)
		if appErr != nil {
			p.API.LogWarn("Error in getting file attachment from Mattermost", "error", appErr)
			continue
		}

		fileName, fileExtension := getFileNameAndExtension(fileInfo.Name)
		var attachment *msteams.Attachment
		attachment, err = client.UploadFile(teamID, channelID, fileName+"_"+fileInfo.Id+fileExtension, int(fileInfo.Size), fileInfo.MimeType, bytes.NewReader(fileData), nil)
		if err != nil {
			p.API.LogWarn("Error in uploading file attachment to MS Teams", "error", err)
			continue
		}
		attachments = append(attachments, attachment)
	}

	md := markdown.New(markdown.XHTMLOutput(true), markdown.Typographer(false))
	content := md.RenderToString([]byte(emoji.Parse(text)))

	content, mentions := p.getMentionsData(content, teamID, channelID, "", client)

	newMessage, err := client.SendMessageWithAttachments(teamID, channelID, parentID, content, attachments, mentions)
	if err != nil {
		p.API.LogError("Error creating post on MS Teams", "error", err.Error())
		return "", err
	}

	if post.Id != "" && newMessage != nil {
		if err := p.store.LinkPosts(storemodels.PostInfo{MattermostID: post.Id, MSTeamsChannel: channelID, MSTeamsID: newMessage.ID, MSTeamsLastUpdateAt: newMessage.LastUpdateAt}, nil); err != nil {
			p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
		}
	}
	return newMessage.ID, nil
}

func (p *Plugin) Delete(teamID, channelID string, user *model.User, post *model.Post) error {
	p.API.LogDebug("Deleting message from MS Teams", "teamID", teamID, "channelID", channelID)

	parentID := ""
	if post.RootId != "" {
		parentInfo, _ := p.store.GetPostInfoByMattermostID(post.RootId)
		if parentInfo != nil {
			parentID = parentInfo.MSTeamsID
		}
	}

	client, err := p.GetClientForUser(user.Id)
	if err != nil {
		client, err = p.GetClientForUser(p.userID)
		if err != nil {
			return err
		}
	}

	postInfo, err := p.store.GetPostInfoByMattermostID(post.Id)
	if err != nil {
		p.API.LogError("Error getting post info", "error", err)
		return err
	}

	if postInfo == nil {
		p.API.LogError("Error deleting post, post not found.")
		return errors.New("post not found")
	}

	if err := client.DeleteMessage(teamID, channelID, parentID, postInfo.MSTeamsID); err != nil {
		p.API.LogError("Error deleting post from MS Teams", "error", err)
		return err
	}

	return nil
}

func (p *Plugin) DeleteChat(chatID string, user *model.User, post *model.Post) error {
	p.API.LogDebug("Deleting direct message from MS Teams", "ChatID", chatID, "PostID", post.Id)

	client, err := p.GetClientForUser(user.Id)
	if err != nil {
		p.handlePromptForConnection(user.Id, post.ChannelId)
		return err
	}

	postInfo, err := p.store.GetPostInfoByMattermostID(post.Id)
	if err != nil {
		p.API.LogError("Error getting post info", "error", err)
		return err
	}

	if postInfo == nil {
		p.API.LogError("Error deleting post, post not found.")
		return errors.New("post not found")
	}

	if err := client.DeleteChatMessage(chatID, postInfo.MSTeamsID); err != nil {
		p.API.LogError("Error deleting post from MS Teams", "error", err)
		return err
	}
	return nil
}

func (p *Plugin) Update(teamID, channelID string, user *model.User, newPost, oldPost *model.Post, updateRequired bool) error {
	p.API.LogDebug("Updating message to MS Teams", "TeamID", teamID, "ChannelID", channelID, "OldPostID", oldPost.Id, "NewPostID", newPost.Id)

	parentID := ""
	if oldPost.RootId != "" {
		parentInfo, _ := p.store.GetPostInfoByMattermostID(newPost.RootId)
		if parentInfo != nil {
			parentID = parentInfo.MSTeamsID
		}
	}

	text := newPost.Message

	client, err := p.GetClientForUser(user.Id)
	if err != nil {
		client, err = p.GetClientForUser(p.userID)
		if err != nil {
			return err
		}
		text = user.Username + ":\n\n" + newPost.Message
	}

	postInfo, err := p.store.GetPostInfoByMattermostID(newPost.Id)
	if err != nil {
		p.API.LogError("Error getting post info", "error", err)
		return err
	}
	if postInfo == nil {
		p.API.LogError("Error updating post, post not found.")
		return errors.New("post not found")
	}

	var updatedMessage *msteams.Message
	tx, err := p.store.BeginTx()
	if err != nil {
		return err
	}

	var txErr error
	defer func() {
		if txErr != nil {
			if err := p.store.RollbackTx(tx); err != nil {
				p.API.LogWarn("Unable to rollback database transaction", "error", err.Error())
			}
			return
		}

		if err := p.store.CommitTx(tx); err != nil {
			p.API.LogWarn("Unable to commit database transaction", "error", err.Error())
		}
	}()

	if txErr = p.store.LockPostByMMPostID(tx, newPost.Id); txErr != nil {
		return txErr
	}

	if updateRequired {
		// TODO: Add the logic of processing the attachments and uploading new files to Teams
		// once Mattermost comes up with the feature of editing attachments
		md := markdown.New(markdown.XHTMLOutput(true), markdown.Typographer(false), markdown.LangPrefix("CodeMirror language-"))
		content := md.RenderToString([]byte(emoji.Parse(text)))
		content, mentions := p.getMentionsData(content, teamID, channelID, "", client)
		updatedMessage, txErr = client.UpdateMessage(teamID, channelID, parentID, postInfo.MSTeamsID, content, mentions)
		if txErr != nil {
			p.API.LogWarn("Error updating the post on MS Teams", "error", txErr)
			// If the error is regarding payment required for metered APIs, ignore it and continue because
			// the post is updated regardless
			if !strings.Contains(txErr.Error(), "code: PaymentRequired") {
				return txErr
			}
		}
	} else {
		updatedMessage, txErr = getUpdatedMessage(teamID, channelID, parentID, postInfo.MSTeamsID, client)
		if txErr != nil {
			p.API.LogWarn("Error in getting the message from MS Teams", "error", txErr)
			return txErr
		}
	}

	if txErr = p.store.LinkPosts(storemodels.PostInfo{MattermostID: newPost.Id, MSTeamsChannel: channelID, MSTeamsID: postInfo.MSTeamsID, MSTeamsLastUpdateAt: updatedMessage.LastUpdateAt}, tx); txErr != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", txErr)
	}

	return nil
}

func (p *Plugin) UpdateChat(chatID string, user *model.User, newPost, oldPost *model.Post, updateRequired bool) error {
	p.API.LogDebug("Updating direct message to MS Teams", "ChatID", chatID, "OldPostID", oldPost.Id, "NewPostID", newPost.Id)

	postInfo, err := p.store.GetPostInfoByMattermostID(newPost.Id)
	if err != nil {
		p.API.LogError("Error getting post info", "error", err)
		return err
	}
	if postInfo == nil {
		p.API.LogError("Error updating post, post not found.")
		return errors.New("post not found")
	}

	text := newPost.Message

	client, err := p.GetClientForUser(user.Id)
	if err != nil {
		p.handlePromptForConnection(user.Id, newPost.ChannelId)
		return err
	}

	var updatedMessage *msteams.Message
	tx, err := p.store.BeginTx()
	if err != nil {
		return err
	}

	var txErr error
	defer func() {
		if txErr != nil {
			if err := p.store.RollbackTx(tx); err != nil {
				p.API.LogWarn("Unable to rollback database transaction", "error", err.Error())
			}
			return
		}

		if err := p.store.CommitTx(tx); err != nil {
			p.API.LogWarn("Unable to commit database transaction", "error", err.Error())
		}
	}()

	if txErr = p.store.LockPostByMMPostID(tx, newPost.Id); txErr != nil {
		return txErr
	}

	if updateRequired {
		md := markdown.New(markdown.XHTMLOutput(true), markdown.Typographer(false), markdown.LangPrefix("CodeMirror language-"))
		content := md.RenderToString([]byte(emoji.Parse(text)))
		content, mentions := p.getMentionsData(content, "", "", chatID, client)
		updatedMessage, txErr = client.UpdateChatMessage(chatID, postInfo.MSTeamsID, content, mentions)
		if txErr != nil {
			p.API.LogWarn("Error updating the post on MS Teams", "error", txErr)
			// If the error is regarding payment required for metered APIs, ignore it and continue because
			// the post is updated regardless
			if !strings.Contains(txErr.Error(), "code: PaymentRequired") {
				return txErr
			}
		}
	} else {
		updatedMessage, txErr = client.GetChatMessage(chatID, postInfo.MSTeamsID)
		if txErr != nil {
			p.API.LogWarn("Error getting the updated message from MS Teams", "error", txErr)
			if !strings.Contains(txErr.Error(), "code: PaymentRequired") {
				return txErr
			}
		}
	}

	if txErr = p.store.LinkPosts(storemodels.PostInfo{MattermostID: newPost.Id, MSTeamsChannel: chatID, MSTeamsID: postInfo.MSTeamsID, MSTeamsLastUpdateAt: updatedMessage.LastUpdateAt}, tx); txErr != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", txErr)
	}

	return nil
}

func (p *Plugin) GetChatIDForChannel(client msteams.Client, channelID string) (string, error) {
	channel, appErr := p.API.GetChannel(channelID)
	if appErr != nil {
		p.API.LogError("Unable to get MM channel", "channelID", channelID, "error", appErr.DetailedError)
		return "", appErr
	}
	if channel.Type != model.ChannelTypeDirect && channel.Type != model.ChannelTypeGroup {
		return "", errors.New("invalid channel type, chatID is only available for direct messages and group messages")
	}

	members, appErr := p.API.GetChannelMembers(channelID, 0, math.MaxInt32)
	if appErr != nil {
		p.API.LogError("Unable to get MM channel members", "channelID", channelID, "error", appErr.DetailedError)
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
		p.API.LogError("Unable to create or get chat for users", "error", err.Error())
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
					p.API.LogDebug("Unable to get MS Teams chat", "Error", err.Error())
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
					p.API.LogDebug("Unable to get MS Teams channel", "Error", err.Error())
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
				p.API.LogDebug("Unable to get user by username", "Error", err.Error())
				continue
			}

			msteamsUserID, getErr := p.store.MattermostToTeamsUserID(mmUser.Id)
			if getErr != nil {
				p.API.LogDebug("Unable to get MS Teams user ID", "Error", getErr.Error())
				continue
			}

			msteamsUser, getErr := client.GetUser(msteamsUserID)
			if getErr != nil {
				p.API.LogDebug("Unable to get MS Teams user", "MSTeamsUserID", msteamsUserID, "Error", getErr.Error())
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

func getUpdatedMessage(teamID, channelID, parentID, msteamsID string, client msteams.Client) (*msteams.Message, error) {
	if parentID != "" {
		return client.GetReply(teamID, channelID, parentID, msteamsID)
	}

	return client.GetMessage(teamID, channelID, msteamsID)
}
