package main

import (
	"bytes"
	"strings"

	"github.com/enescakir/emoji"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
)

func (p *Plugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
	if len(post.FileIds) > 0 && p.configuration.SyncDirectMessages {
		channel, err := p.API.GetChannel(post.ChannelId)
		if err != nil {
			return post, ""
		}
		if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
			members, err := p.API.GetChannelMembers(channel.Id, 0, 10)
			if err != nil {
				return post, ""
			}
			for _, member := range members {
				user, err := p.API.GetUser(member.UserId)
				if err != nil {
					return post, ""
				}
				if strings.HasSuffix(user.Email, "@msteamssync") {
					p.API.SendEphemeralPost(post.UserId, &model.Post{
						Message:   "Attachments not supported in direct messages with MSTeams members",
						UserId:    p.userID,
						ChannelId: channel.Id,
					})
					return nil, "Attachments not supported in direct messages with MSTeams members"
				}
			}
		}
	}
	return post, ""
}

func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	p.API.LogDebug("Create message hook", "post", post)
	if post.Props != nil {
		if _, ok := post.Props["msteams_sync_"+p.userID].(bool); ok {
			return
		}
	}

	link, err := p.store.GetLinkByChannelID(post.ChannelId)
	if err != nil || link == nil {
		channel, appErr := p.API.GetChannel(post.ChannelId)
		if appErr != nil {
			return
		}
		if (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) && p.configuration.SyncDirectMessages {
			members, appErr := p.API.GetChannelMembers(post.ChannelId, 0, 10)
			if appErr != nil {
				return
			}
			dstUsers := []string{}
			for _, m := range members {
				dstUsers = append(dstUsers, m.UserId)
			}
			_, err = p.SendChat(post.UserId, dstUsers, post)
			if err != nil {
				p.API.LogError("Unable to handle message sent", "error", err.Error())
			}
		}
		return
	}

	user, _ := p.API.GetUser(post.UserId)

	_, err = p.Send(link.MSTeamsTeam, link.MSTeamsChannel, user, post)
	if err != nil {
		p.API.LogError("Unable to handle message sent", "error", err.Error())
	}
}

func (p *Plugin) ReactionHasBeenAdded(c *plugin.Context, reaction *model.Reaction) {
	postInfo, err := p.store.GetPostInfoByMattermostID(reaction.PostId)
	if err != nil || postInfo == nil {
		return
	}

	link, err := p.store.GetLinkByChannelID(reaction.ChannelId)
	if err != nil || link == nil {
		channel, appErr := p.API.GetChannel(reaction.ChannelId)
		if appErr != nil {
			return
		}
		if (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) && p.configuration.SyncDirectMessages {
			err = p.SetChatReaction(postInfo.MSTeamsID, reaction.UserId, reaction.ChannelId, reaction.EmojiName)
			if err != nil {
				p.API.LogError("Unable to handle message reaction set", "error", err.Error())
			}
		}
		return
	}

	post, appErr := p.API.GetPost(reaction.PostId)
	if appErr != nil {
		p.API.LogError("Unable to get the post from the reaction", "reaction", reaction, "error", appErr)
		return
	}

	user, _ := p.API.GetUser(post.UserId)

	err = p.SetReaction(link.MSTeamsTeam, link.MSTeamsChannel, user, post, reaction.EmojiName)
	if err != nil {
		p.API.LogError("Unable to handle message reaction set", "error", err.Error())
	}
}

func (p *Plugin) ReactionHasBeenRemoved(c *plugin.Context, reaction *model.Reaction) {
	p.API.LogError("Removing reaction hook", "reaction", reaction)
	if reaction.ChannelId == "removefromplugin" {
		p.API.LogError("Ignore reaction that has been trigger from the plugin handler")
		return
	}
	postInfo, err := p.store.GetPostInfoByMattermostID(reaction.PostId)
	if err != nil || postInfo == nil {
		return
	}

	post, appErr := p.API.GetPost(reaction.PostId)
	if appErr != nil {
		p.API.LogError("Unable to get the post from the reaction", "reaction", reaction, "error", appErr)
		return
	}

	link, err := p.store.GetLinkByChannelID(post.ChannelId)
	if err != nil || link == nil {
		channel, appErr := p.API.GetChannel(post.ChannelId)
		if appErr != nil {
			return
		}
		if (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) && p.configuration.SyncDirectMessages {
			err = p.UnsetChatReaction(postInfo.MSTeamsID, reaction.UserId, post.ChannelId, reaction.EmojiName)
			if err != nil {
				p.API.LogError("Unable to handle message reaction unset", "error", err.Error())
			}
		}
		return
	}

	user, _ := p.API.GetUser(post.UserId)

	err = p.UnsetReaction(link.MSTeamsTeam, link.MSTeamsChannel, user, post, reaction.EmojiName)
	if err != nil {
		p.API.LogError("Unable to handle message reaction unset", "error", err.Error())
	}
}

func (p *Plugin) MessageHasBeenUpdated(c *plugin.Context, newPost, oldPost *model.Post) {
	p.API.LogDebug("Updating message hook", "newPost", newPost, "oldPost", oldPost)
	if oldPost.Props != nil {
		if _, ok := oldPost.Props["msteams_sync_"+p.userID].(bool); ok {
			return
		}
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
		if !p.configuration.SyncDirectMessages {
			return
		}

		members, appErr := p.API.GetChannelMembers(newPost.ChannelId, 0, 10)
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
		var chatID string
		chatID, err = client.CreateOrGetChatForUsers(usersIDs)
		if err != nil {
			return
		}
		err = p.UpdateChat(chatID, user, newPost, oldPost)
		if err != nil {
			p.API.LogError("Unable to handle message update", "error", err.Error())
		}
		return
	}

	err = p.Update(link.MSTeamsTeam, link.MSTeamsChannel, user, newPost, oldPost)
	if err != nil {
		p.API.LogError("Unable to handle message update", "error", err.Error())
	}
}

func (p *Plugin) SetChatReaction(teamsMessageID, srcUser, channelID string, emojiName string) error {
	p.API.LogDebug("Setting chat reaction", "srcUser", srcUser, "emojiName", emojiName, "channelID", channelID)

	srcUserID, err := p.store.MattermostToTeamsUserID(srcUser)
	if err != nil {
		return err
	}

	client, err := p.GetClientForUser(srcUser)
	if err != nil {
		return err
	}

	chatID, err := p.GetChatIDForChannel(srcUser, channelID)
	if err != nil {
		return err
	}

	err = client.SetChatReaction(chatID, teamsMessageID, srcUserID, emoji.Parse(":"+emojiName+":"))
	if err != nil {
		p.API.LogWarn("Error creating post reaction", "error", err.Error())
		return err
	}

	return nil
}

func (p *Plugin) SetReaction(teamID, channelID string, user *model.User, post *model.Post, emojiName string) error {
	p.API.LogDebug("Setting reaction", "teamID", teamID, "channelID", channelID, "post", post, "emojiName", emojiName)

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

	client, err := p.GetClientForUser(user.Id)
	if err != nil {
		client, err = p.GetClientForUser(p.userID)
		if err != nil {
			return err
		}
	}

	err = client.SetReaction(teamID, channelID, parentID, postInfo.MSTeamsID, user.Id, emoji.Parse(":"+emojiName+":"))
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err.Error())
		return err
	}

	return nil
}

func (p *Plugin) UnsetChatReaction(teamsMessageID, srcUser, channelID string, emojiName string) error {
	p.API.LogDebug("Unsetting chat reaction", "srcUser", srcUser, "emojiName", emojiName, "channelID", channelID)

	srcUserID, err := p.store.MattermostToTeamsUserID(srcUser)
	if err != nil {
		return err
	}

	client, err := p.GetClientForUser(srcUser)
	if err != nil {
		return err
	}

	chatID, err := p.GetChatIDForChannel(srcUser, channelID)
	if err != nil {
		p.API.LogError("FAILING TO CREATE OR GET THE CHAT", "error", err)
		return err
	}

	err = client.UnsetChatReaction(chatID, teamsMessageID, srcUserID, emoji.Parse(":"+emojiName+":"))
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err.Error())
		return err
	}

	return nil
}

func (p *Plugin) UnsetReaction(teamID, channelID string, user *model.User, post *model.Post, emojiName string) error {
	p.API.LogDebug("Unsetting reaction", "teamID", teamID, "channelID", channelID, "post", post, "emojiName", emojiName)

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

	client, err := p.GetClientForUser(user.Id)
	if err != nil {
		client, err = p.GetClientForUser(p.userID)
		if err != nil {
			return err
		}
	}

	err = client.UnsetReaction(teamID, channelID, parentID, postInfo.MSTeamsID, user.Id, emoji.Parse(":"+emojiName+":"))
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err.Error())
		return err
	}

	return nil
}
func (p *Plugin) SendChat(srcUser string, usersIDs []string, post *model.Post) (string, error) {
	p.API.LogDebug("Sending direct message to MS Teams", "srcUser", srcUser, "usersIDs", usersIDs, "post", post)

	parentID := ""
	if post.RootId != "" {
		parentInfo, _ := p.store.GetPostInfoByMattermostID(post.RootId)
		if parentInfo != nil {
			parentID = parentInfo.MSTeamsID
		}
	}

	srcUserID, err := p.store.MattermostToTeamsUserID(srcUser)
	if err != nil {
		return "", err
	}
	teamsUsersIDs := make([]string, len(usersIDs))
	for idx, userID := range usersIDs {
		var teamsUserID string
		teamsUserID, err = p.store.MattermostToTeamsUserID(userID)
		if err != nil {
			return "", err
		}
		teamsUsersIDs[idx] = teamsUserID
	}

	p.API.LogDebug("Sending direct message to MS Teams", "srcUserID", srcUserID, "teamsUsersIDs", teamsUsersIDs, "post", post)
	text := post.Message

	client, err := p.GetClientForUser(srcUser)
	if err != nil {
		return "", err
	}

	chatID, err := client.CreateOrGetChatForUsers(teamsUsersIDs)
	if err != nil {
		p.API.LogError("FAILING TO CREATE OR GET THE CHAT", "error", err)
		return "", err
	}

	newMessage, err := client.SendChat(chatID, parentID, text)
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err.Error())
		return "", err
	}

	if post.Id != "" && newMessage != nil {
		err := p.store.LinkPosts(storemodels.PostInfo{MattermostID: post.Id, MSTeamsChannel: chatID, MSTeamsID: newMessage.ID, MSTeamsLastUpdateAt: newMessage.LastUpdateAt})
		if err != nil {
			p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
		}
	}
	return newMessage.ID, nil
}

func (p *Plugin) Send(teamID, channelID string, user *model.User, post *model.Post) (string, error) {
	p.API.LogDebug("Sending message to MS Teams", "teamID", teamID, "channelID", channelID, "post", post)

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
			p.API.LogWarn("Unable to get file attachment", "error", appErr)
			continue
		}
		fileData, appErr := p.API.GetFile(fileInfo.Id)
		if appErr != nil {
			p.API.LogWarn("error get file attachment from mattermost", "error", appErr)
			continue
		}

		var attachment *msteams.Attachment
		attachment, err = client.UploadFile(teamID, channelID, fileInfo.Id+"_"+fileInfo.Name, int(fileInfo.Size), fileInfo.MimeType, bytes.NewReader(fileData))
		if err != nil {
			p.API.LogWarn("error uploading attachment", "error", err)
			continue
		}
		attachments = append(attachments, attachment)
	}

	newMessage, err := client.SendMessageWithAttachments(teamID, channelID, parentID, text, attachments)
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err.Error())
		return "", err
	}

	if post.Id != "" && newMessage != nil {
		err := p.store.LinkPosts(storemodels.PostInfo{MattermostID: post.Id, MSTeamsChannel: channelID, MSTeamsID: newMessage.ID, MSTeamsLastUpdateAt: newMessage.LastUpdateAt})
		if err != nil {
			p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
		}
	}
	return newMessage.ID, nil
}

func (p *Plugin) Delete(teamID, channelID string, user *model.User, post *model.Post) error {
	p.API.LogDebug("Deleting message to MS Teams", "teamID", teamID, "channelID", channelID, "post", post)

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
		p.API.LogError("Error updating post", "error", err)
		return err
	}
	if postInfo == nil {
		p.API.LogError("Error deleting post, post not found.")
		return errors.New("post not found")
	}

	if err := client.DeleteMessage(teamID, channelID, parentID, postInfo.MSTeamsID); err != nil {
		p.API.LogError("Error deleting post", "error", err)
		return err
	}

	return nil
}

func (p *Plugin) DeleteChat(chatID string, user *model.User, post *model.Post) error {
	p.API.LogDebug("Deleting direct message to MS Teams", "chatID", chatID, "post", post)

	client, err := p.GetClientForUser(user.Id)
	if err != nil {
		client, err = p.GetClientForUser(p.userID)
		if err != nil {
			return err
		}
	}

	postInfo, err := p.store.GetPostInfoByMattermostID(post.Id)
	if err != nil {
		p.API.LogError("Error updating post", "error", err)
		return err
	}
	if postInfo == nil {
		p.API.LogError("Error deleting post, post not found.")
		return errors.New("post not found")
	}

	if err := client.DeleteChatMessage(chatID, postInfo.MSTeamsID); err != nil {
		p.API.LogError("Error deleting post", "error", err)
		return err
	}
	return nil
}

func (p *Plugin) Update(teamID, channelID string, user *model.User, newPost, oldPost *model.Post) error {
	p.API.LogDebug("Updating message to MS Teams", "teamID", teamID, "channelID", channelID, "oldPost", oldPost, "newPost", newPost)

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
		p.API.LogError("Error updating post", "error", err)
		return err
	}
	if postInfo == nil {
		p.API.LogError("Error updating post, post not found.")
		return errors.New("post not found")
	}

	if err = client.UpdateMessage(teamID, channelID, parentID, postInfo.MSTeamsID, text); err != nil {
		p.API.LogWarn("Error updating the post", "error", err)
		return err
	}

	var updatedMessage *msteams.Message
	if parentID != "" {
		updatedMessage, err = client.GetReply(teamID, channelID, parentID, postInfo.MSTeamsID)
	} else {
		updatedMessage, err = client.GetMessage(teamID, channelID, postInfo.MSTeamsID)
	}
	if err != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
		return nil
	}

	if err = p.store.LinkPosts(storemodels.PostInfo{MattermostID: newPost.Id, MSTeamsChannel: channelID, MSTeamsID: postInfo.MSTeamsID, MSTeamsLastUpdateAt: updatedMessage.LastUpdateAt}); err != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
	}

	return nil
}

func (p *Plugin) UpdateChat(chatID string, user *model.User, newPost, oldPost *model.Post) error {
	p.API.LogDebug("Updating direct message to MS Teams", "chatID", chatID, "oldPost", oldPost, "newPost", newPost)

	postInfo, err := p.store.GetPostInfoByMattermostID(newPost.Id)
	if err != nil {
		p.API.LogError("Error updating post", "error", err)
		return err
	}
	if postInfo == nil {
		p.API.LogError("Error updating post, post not found.")
		return errors.New("post not found")
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

	if err = client.UpdateChatMessage(chatID, postInfo.MSTeamsID, text); err != nil {
		p.API.LogWarn("Error updating the post", "error", err)
		return err
	}

	updatedMessage, err := client.GetChatMessage(chatID, postInfo.MSTeamsID)
	if err != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
	} else {
		err := p.store.LinkPosts(storemodels.PostInfo{MattermostID: newPost.Id, MSTeamsChannel: chatID, MSTeamsID: postInfo.MSTeamsID, MSTeamsLastUpdateAt: updatedMessage.LastUpdateAt})
		if err != nil {
			p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
		}
	}

	return nil
}

func (p *Plugin) GetChatIDForChannel(clientUserID string, channelID string) (string, error) {
	channel, appErr := p.API.GetChannel(channelID)
	if appErr != nil {
		return "", appErr
	}
	if channel.Type != model.ChannelTypeDirect && channel.Type != model.ChannelTypeGroup {
		return "", errors.New("invalid channel type, chatID is only available for direct messages and group messages")
	}

	members, appErr := p.API.GetChannelMembers(channelID, 0, 10)
	if appErr != nil {
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
	client, err := p.GetClientForUser(clientUserID)
	if err != nil {
		return "", err
	}
	chatID, err := client.CreateOrGetChatForUsers(teamsUsersIDs)
	if err != nil {
		return "", err
	}
	return chatID, nil
}
