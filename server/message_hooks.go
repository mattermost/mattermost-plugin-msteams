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
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/pkg/errors"
	"gitlab.com/golang-commonmark/markdown"
)

func (p *Plugin) UserWillLogIn(_ *plugin.Context, user *model.User) string {
	if user.RemoteId != nil && *user.RemoteId != "" && p.getConfiguration().AutomaticallyPromoteSyntheticUsers {
		*user.RemoteId = ""
		if _, appErr := p.API.UpdateUser(user); appErr != nil {
			p.API.LogWarn("Unable to promote synthetic user", "user_id", user.Id, "error", appErr.Error())
			return "Unable to promote synthetic user"
		}

		p.API.LogInfo("Promoted synthetic user", "user_id", user.Id)
	}

	return ""
}

func (p *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	if post.Props != nil {
		if _, ok := post.Props["msteams_sync_"+p.userID].(bool); ok {
			return
		}
	}

	if post.IsSystemMessage() {
		return
	}

	link, err := p.store.GetLinkByChannelID(post.ChannelId)
	if err != nil || link == nil {
		channel, appErr := p.API.GetChannel(post.ChannelId)
		if appErr != nil {
			return
		}
		if (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) && p.getConfiguration().SyncDirectMessages {
			p.GetMetrics().ObserveMessageHooksEvent("message_create")

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
	p.GetMetrics().ObserveMessageHooksEvent("message_create")

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

	postInfo, err := p.store.GetPostInfoByMattermostID(reaction.PostId)
	if err != nil {
		p.API.LogWarn("Failed to find Teams post corresponding to MM post", "post_id", reaction.PostId, "error", err.Error())
	} else if postInfo == nil {
		return
	}

	link, err := p.store.GetLinkByChannelID(reaction.ChannelId)
	if err != nil || link == nil {
		channel, appErr := p.API.GetChannel(reaction.ChannelId)
		if appErr != nil {
			return
		}
		if (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) && p.getConfiguration().SyncDirectMessages {
			p.GetMetrics().ObserveMessageHooksEvent("reaction_added")
			err = p.SetChatReaction(postInfo.MSTeamsID, reaction.UserId, reaction.ChannelId, reaction.EmojiName, updateRequired)
			if err != nil {
				p.API.LogWarn("Unable to handle message reaction set", "error", err.Error())
			}
		}
		return
	}

	p.GetMetrics().ObserveMessageHooksEvent("reaction_added")

	post, appErr := p.API.GetPost(reaction.PostId)
	if appErr != nil {
		p.API.LogWarn("Unable to get the post from the reaction", "reaction", reaction, "error", appErr)
		return
	}

	if err = p.SetReaction(link.MSTeamsTeam, link.MSTeamsChannel, reaction.UserId, post, reaction.EmojiName, updateRequired); err != nil {
		p.API.LogWarn("Unable to handle message reaction set", "error", err.Error())
	}
}

func (p *Plugin) ReactionHasBeenRemoved(_ *plugin.Context, reaction *model.Reaction) {
	if reaction.ChannelId == "removedfromplugin" {
		return
	}
	postInfo, err := p.store.GetPostInfoByMattermostID(reaction.PostId)
	if err != nil {
		p.API.LogWarn("Failed to find Teams post corresponding to MM post", "post_id", reaction.PostId, "error", err.Error())
	} else if postInfo == nil {
		return
	}

	post, appErr := p.API.GetPost(reaction.PostId)
	if appErr != nil {
		p.API.LogWarn("Unable to get the post from the reaction", "reaction", reaction, "error", appErr.DetailedError)
		return
	}

	link, err := p.store.GetLinkByChannelID(post.ChannelId)
	if err != nil || link == nil {
		channel, appErr := p.API.GetChannel(post.ChannelId)
		if appErr != nil {
			return
		}
		if (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) && p.getConfiguration().SyncDirectMessages {
			p.GetMetrics().ObserveMessageHooksEvent("reaction_removed")
			err = p.UnsetChatReaction(postInfo.MSTeamsID, reaction.UserId, post.ChannelId, reaction.EmojiName)
			if err != nil {
				p.API.LogWarn("Unable to handle chat message reaction unset", "error", err.Error())
			}
		}
		return
	}
	p.GetMetrics().ObserveMessageHooksEvent("reaction_removed")

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
		p.GetMetrics().ObserveMessageHooksEvent("message_update")

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
		var chat *clientmodels.Chat
		chat, err = client.CreateOrGetChatForUsers(usersIDs)
		if err != nil {
			p.API.LogWarn("Unable to create or get chat for users", "error", err.Error())
			return
		}
		err = p.UpdateChat(chat.ID, user, newPost, updateRequired)
		if err != nil {
			p.API.LogWarn("Unable to handle message update", "error", err.Error())
		}
		return
	}
	p.GetMetrics().ObserveMessageHooksEvent("message_update")

	err = p.Update(link.MSTeamsTeam, link.MSTeamsChannel, user, newPost, updateRequired)
	if err != nil {
		p.API.LogWarn("Unable to handle message update", "error", err.Error())
	}
}

func (p *Plugin) SetChatReaction(teamsMessageID, srcUser, channelID, emojiName string, updateRequired bool) error {
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

	var teamsMessage *clientmodels.Message

	mutex, err := cluster.NewMutex(p.API, "post_mutex_"+chatID+teamsMessageID)
	if err != nil {
		return err
	}
	mutex.Lock()
	defer mutex.Unlock()

	if updateRequired {
		teamsMessage, err = client.SetChatReaction(chatID, teamsMessageID, srcUserID, emoji.Parse(":"+emojiName+":"))
		if err != nil {
			p.API.LogWarn("Error creating post reaction", "error", err.Error())
			return err
		}

		p.GetMetrics().ObserveReaction(metrics.ReactionSetAction, metrics.ActionSourceMattermost, true)
	} else {
		teamsMessage, err = client.GetChatMessage(chatID, teamsMessageID)
		if err != nil {
			p.API.LogWarn("Error getting the msteams post metadata", "error", err.Error())
			return err
		}
	}

	if err = p.store.SetPostLastUpdateAtByMSTeamsID(teamsMessageID, teamsMessage.LastUpdateAt); err != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err.Error())
	}

	return nil
}

func (p *Plugin) SetReaction(teamID, channelID, userID string, post *model.Post, emojiName string, updateRequired bool) error {
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

	var teamsMessage *clientmodels.Message

	mutex, err := cluster.NewMutex(p.API, "post_mutex_"+post.Id)
	if err != nil {
		return err
	}
	mutex.Lock()
	defer mutex.Unlock()

	if updateRequired {
		teamsUserID, _ := p.store.MattermostToTeamsUserID(userID)
		teamsMessage, err = client.SetReaction(teamID, channelID, parentID, postInfo.MSTeamsID, teamsUserID, emoji.Parse(":"+emojiName+":"))
		if err != nil {
			p.API.LogWarn("Error setting reaction", "error", err.Error())
			return err
		}

		p.GetMetrics().ObserveReaction(metrics.ReactionSetAction, metrics.ActionSourceMattermost, false)
	} else {
		teamsMessage, err = getUpdatedMessage(teamID, channelID, parentID, postInfo.MSTeamsID, client)
		if err != nil {
			p.API.LogWarn("Error getting the msteams post metadata", "error", err.Error())
			return err
		}
	}

	if err = p.store.SetPostLastUpdateAtByMattermostID(postInfo.MattermostID, teamsMessage.LastUpdateAt); err != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err.Error())
	}

	return nil
}

func (p *Plugin) UnsetChatReaction(teamsMessageID, srcUser, channelID string, emojiName string) error {
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

	mutex, err := cluster.NewMutex(p.API, "post_mutex_"+chatID+teamsMessageID)
	if err != nil {
		return err
	}
	mutex.Lock()
	defer mutex.Unlock()

	teamsMessage, err := client.UnsetChatReaction(chatID, teamsMessageID, srcUserID, emoji.Parse(":"+emojiName+":"))
	if err != nil {
		p.API.LogWarn("Error in removing the chat reaction", "emoji_name", emojiName, "error", err.Error())
		return err
	}

	p.GetMetrics().ObserveReaction(metrics.ReactionUnsetAction, metrics.ActionSourceMattermost, true)
	if err = p.store.SetPostLastUpdateAtByMSTeamsID(teamsMessageID, teamsMessage.LastUpdateAt); err != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err.Error())
	}

	return nil
}

func (p *Plugin) UnsetReaction(teamID, channelID, userID string, post *model.Post, emojiName string) error {
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

	mutex, err := cluster.NewMutex(p.API, "post_mutex_"+post.Id)
	if err != nil {
		return err
	}
	mutex.Lock()
	defer mutex.Unlock()

	teamsMessage, err := client.UnsetReaction(teamID, channelID, parentID, postInfo.MSTeamsID, teamsUserID, emoji.Parse(":"+emojiName+":"))
	if err != nil {
		p.API.LogWarn("Error in removing the reaction", "emoji_name", emojiName, "error", err.Error())
		return err
	}

	p.GetMetrics().ObserveReaction(metrics.ReactionUnsetAction, metrics.ActionSourceMattermost, false)
	if err = p.store.SetPostLastUpdateAtByMattermostID(postInfo.MattermostID, teamsMessage.LastUpdateAt); err != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err.Error())
	}

	return nil
}

func (p *Plugin) SendChat(srcUser string, usersIDs []string, post *model.Post) (string, error) {
	parentID := ""
	if post.RootId != "" {
		parentInfo, _ := p.store.GetPostInfoByMattermostID(post.RootId)
		if parentInfo != nil {
			parentID = parentInfo.MSTeamsID
		}
	}

	_, err := p.store.MattermostToTeamsUserID(srcUser)
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
		p.GetMetrics().ObserveMessageHooksEvent("attachment_created")
		p.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMattermost, "", true)
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

	newMessage, err := client.SendChat(chat.ID, content, parentMessage, attachments, mentions)
	if err != nil {
		p.API.LogWarn("Error creating post on MS Teams", "error", err.Error())
		return "", err
	}

	p.GetMetrics().ObserveMessage(metrics.ActionCreated, metrics.ActionSourceMattermost, true)
	if post.Id != "" {
		if err := p.store.LinkPosts(storemodels.PostInfo{MattermostID: post.Id, MSTeamsChannel: chat.ID, MSTeamsID: newMessage.ID, MSTeamsLastUpdateAt: newMessage.LastUpdateAt}); err != nil {
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
		p.API.LogWarn("Unable to get the last prompt timestamp for the channel", "channel_id", channelID, "error", err.Error())
	}

	if time.Until(timestamp) < -time.Hour*time.Duration(promptInterval) {
		p.sendBotEphemeralPost(userID, channelID, "Your Mattermost account is not connected to MS Teams so your activity will not be relayed to users on MS Teams. You can connect your account using the `/msteams-sync connect` slash command.")

		if err = p.store.StoreDMAndGMChannelPromptTime(channelID, userID, time.Now()); err != nil {
			p.API.LogWarn("Unable to store the last prompt timestamp for the channel", "channel_id", channelID, "error", err.Error())
		}
	}
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
		client, err = p.GetClientForUser(p.userID)
		if err != nil {
			return "", err
		}
		text = user.Username + ":\n\n" + post.Message
	}

	var attachments []*clientmodels.Attachment
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
		p.GetMetrics().ObserveMessageHooksEvent("attachment_created")
		p.GetMetrics().ObserveFile(metrics.ActionCreated, metrics.ActionSourceMattermost, "", false)
	}

	md := markdown.New(markdown.XHTMLOutput(true), markdown.Typographer(false))
	content := md.RenderToString([]byte(emoji.Parse(text)))

	content, mentions := p.getMentionsData(content, teamID, channelID, "", client)

	newMessage, err := client.SendMessageWithAttachments(teamID, channelID, parentID, content, attachments, mentions)
	if err != nil {
		p.API.LogWarn("Error creating post on MS Teams", "error", err.Error())
		return "", err
	}

	p.GetMetrics().ObserveMessage(metrics.ActionCreated, metrics.ActionSourceMattermost, false)
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
		client, err = p.GetClientForUser(p.userID)
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

func (p *Plugin) DeleteChat(chatID string, user *model.User, post *model.Post) error {
	client, err := p.GetClientForUser(user.Id)
	if err != nil {
		p.handlePromptForConnection(user.Id, post.ChannelId)
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

	if err := client.DeleteChatMessage(chatID, postInfo.MSTeamsID); err != nil {
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
		client, err = p.GetClientForUser(p.userID)
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
		p.handlePromptForConnection(user.Id, newPost.ChannelId)
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
	if channel.Type != model.ChannelTypeDirect && channel.Type != model.ChannelTypeGroup {
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
