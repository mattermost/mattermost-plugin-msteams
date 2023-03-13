package main

import (
	"bytes"
	"strings"

	"github.com/enescakir/emoji"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
)

func (p *Plugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
	p.API.LogError("Message will be posted hook", "post", post)
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
	p.API.LogError("Create message hook", "post", post)
	if post.Props != nil {
		if _, ok := post.Props["msteams_sync_"+p.userID].(bool); ok {
			return
		}
	}

	link, err := p.store.GetLinkByChannelID(post.ChannelId)
	if err != nil || link == nil {
		channel, err := p.API.GetChannel(post.ChannelId)
		if err != nil {
			return
		}
		if (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) && p.configuration.SyncDirectMessages {
			members, err := p.API.GetChannelMembers(post.ChannelId, 0, 10)
			if err != nil {
				return
			}
			dstUsers := []string{}
			for _, m := range members {
				dstUsers = append(dstUsers, m.UserId)
			}
			p.SendChat(post.UserId, dstUsers, post)
		}
		return
	}

	user, _ := p.API.GetUser(post.UserId)

	p.Send(link.MSTeamsTeam, link.MSTeamsChannel, user, post)
}

func (p *Plugin) ReactionHasBeenAdded(c *plugin.Context, reaction *model.Reaction) {
	p.API.LogError("Adding reaction hook", "reaction", reaction)
	teamsMessageID, err := p.store.MattermostToTeamsPostID(reaction.PostId)
	if err != nil || teamsMessageID == "" {
		return
	}

	link, err := p.store.GetLinkByChannelID(reaction.ChannelId)
	if err != nil || link == nil {
		channel, err := p.API.GetChannel(reaction.ChannelId)
		if err != nil {
			return
		}
		if (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) && p.configuration.SyncDirectMessages {
			p.SetChatReaction(teamsMessageID, reaction.UserId, reaction.ChannelId, reaction.EmojiName)
			return
		}
		return
	}

	post, appErr := p.API.GetPost(reaction.PostId)
	if appErr != nil {
		p.API.LogError("Unable to get the post from the reaction", "reaction", reaction, "error", appErr)
		return
	}

	user, _ := p.API.GetUser(post.UserId)

	p.SetReaction(link.MSTeamsTeam, link.MSTeamsChannel, user, post, reaction.EmojiName)
}

func (p *Plugin) ReactionHasBeenRemoved(c *plugin.Context, reaction *model.Reaction) {
	p.API.LogError("Removing reaction hook", "reaction", reaction)
	if reaction.ChannelId == "removefromplugin" {
		p.API.LogError("Ignore reaction that has been trigger from the plugin handler")
		return
	}
	teamsMessageID, err := p.store.MattermostToTeamsPostID(reaction.PostId)
	if err != nil || teamsMessageID == "" {
		return
	}

	post, appErr := p.API.GetPost(reaction.PostId)
	if appErr != nil {
		p.API.LogError("Unable to get the post from the reaction", "reaction", reaction, "error", appErr)
		return
	}

	link, err := p.store.GetLinkByChannelID(post.ChannelId)
	if err != nil || link == nil {
		channel, err := p.API.GetChannel(post.ChannelId)
		if err != nil {
			return
		}
		if (channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup) && p.configuration.SyncDirectMessages {
			p.UnsetChatReaction(teamsMessageID, reaction.UserId, post.ChannelId, reaction.EmojiName)
			return
		}
		return
	}

	user, _ := p.API.GetUser(post.UserId)

	p.UnsetReaction(link.MSTeamsTeam, link.MSTeamsChannel, user, post, reaction.EmojiName)
}

func (p *Plugin) MessageHasBeenUpdated(c *plugin.Context, newPost, oldPost *model.Post) {
	p.API.LogDebug("Updating message hook", "newPost", newPost, "oldPost", oldPost)
	if oldPost.Props != nil {
		if _, ok := oldPost.Props["msteams_sync_"+p.userID].(bool); ok {
			return
		}
	}

	client, err := p.getClientForUser(newPost.UserId)
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
			teamsUserID, err := p.store.MattermostToTeamsUserID(m.UserId)
			if err != nil {
				return
			}
			usersIDs = append(usersIDs, teamsUserID)
		}
		chatID, err := client.CreateOrGetChatForUsers(usersIDs)
		if err != nil {
			return
		}
		p.UpdateChat(chatID, user, newPost, oldPost)
		return
	}

	p.Update(link.MSTeamsTeam, link.MSTeamsChannel, user, newPost, oldPost)
}

func (p *Plugin) SetChatReaction(teamsMessageID, srcUser, channelID string, emojiName string) error {
	p.API.LogDebug("Setting chat reaction", "srcUser", srcUser, "emojiName", emojiName, "channelID", channelID)

	srcUserID, err := p.store.MattermostToTeamsUserID(srcUser)
	if err != nil {
		return err
	}

	client, err := p.getClientForUser(srcUser)
	if err != nil {
		return err
	}

	chatID, err := p.GetChatIDForChannel(srcUser, channelID)
	if err != nil {
		p.API.LogError("FAILING TO CREATE OR GET THE CHAT", "error", err)
		return err
	}

	p.API.LogError("EMOJI AND EMOJI UNICODE", "emojiName", emojiName, "emojiUnicode", emoji.Parse(":"+emojiName+":"))
	err = client.SetChatReaction(chatID, teamsMessageID, srcUserID, emoji.Parse(":"+emojiName+":"))
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err)
		return err
	}

	return nil
}

func (p *Plugin) SetReaction(teamID, channelID string, user *model.User, post *model.Post, emojiName string) error {
	p.API.LogDebug("Setting reaction", "teamID", teamID, "channelID", channelID, "post", post, "emojiName", emojiName)

	teamsMessageID, err := p.store.MattermostToTeamsPostID(post.Id)
	if err != nil {
		return err
	}

	if teamsMessageID == "" {
		return errors.New("teams message not found")
	}

	parentID := ""
	if post.RootId != "" {
		parentID, _ = p.store.MattermostToTeamsPostID(post.RootId)
	}

	client, err := p.getClientForUser(user.Id)
	if err != nil {
		client = p.msteamsBotClient
	}

	p.API.LogError("EMOJI AND EMOJI UNICODE", "emojiName", emojiName, "emojiUnicode", emoji.Parse(":"+emojiName+":"))
	err = client.SetReaction(teamID, channelID, parentID, teamsMessageID, user.Id, emoji.Parse(":"+emojiName+":"))
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err)
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

	client, err := p.getClientForUser(srcUser)
	if err != nil {
		return err
	}

	chatID, err := p.GetChatIDForChannel(srcUser, channelID)
	if err != nil {
		p.API.LogError("FAILING TO CREATE OR GET THE CHAT", "error", err)
		return err
	}

	p.API.LogError("EMOJI AND EMOJI UNICODE", "emojiName", emojiName, "emojiUnicode", emoji.Parse(":"+emojiName+":"))
	err = client.UnsetChatReaction(chatID, teamsMessageID, srcUserID, emoji.Parse(":"+emojiName+":"))
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err)
		return err
	}

	return nil
}

func (p *Plugin) UnsetReaction(teamID, channelID string, user *model.User, post *model.Post, emojiName string) error {
	p.API.LogDebug("Unsetting reaction", "teamID", teamID, "channelID", channelID, "post", post, "emojiName", emojiName)

	teamsMessageID, err := p.store.MattermostToTeamsPostID(post.Id)
	if err != nil {
		return err
	}

	if teamsMessageID == "" {
		return errors.New("teams message not found")
	}

	parentID := ""
	if post.RootId != "" {
		parentID, _ = p.store.MattermostToTeamsPostID(post.RootId)
	}

	client, err := p.getClientForUser(user.Id)
	if err != nil {
		client = p.msteamsBotClient
	}

	p.API.LogError("EMOJI AND EMOJI UNICODE", "emojiName", emojiName, "emojiUnicode", emoji.Parse(":"+emojiName+":"))
	err = client.UnsetReaction(teamID, channelID, parentID, teamsMessageID, user.Id, emoji.Parse(":"+emojiName+":"))
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err)
		return err
	}

	return nil
}
func (p *Plugin) SendChat(srcUser string, usersIDs []string, post *model.Post) (string, error) {
	p.API.LogDebug("Sending direct message to MS Teams", "srcUser", srcUser, "usersIDs", usersIDs, "post", post)

	parentID := ""
	if post.RootId != "" {
		parentID, _ = p.store.MattermostToTeamsPostID(post.RootId)
	}

	srcUserID, err := p.store.MattermostToTeamsUserID(srcUser)
	if err != nil {
		return "", err
	}
	teamsUsersIDs := make([]string, len(usersIDs))
	for idx, userID := range usersIDs {
		teamsUserID, err := p.store.MattermostToTeamsUserID(userID)
		if err != nil {
			return "", err
		}
		teamsUsersIDs[idx] = teamsUserID
	}

	p.API.LogDebug("Sending direct message to MS Teams", "srcUserID", srcUserID, "teamsUsersIDs", teamsUsersIDs, "post", post)
	text := post.Message

	client, err := p.getClientForUser(srcUser)
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
		p.API.LogWarn("Error creating post", "error", err)
		return "", err
	}

	if post.Id != "" && newMessage != nil {
		err := p.store.LinkPosts(post.Id, chatID, newMessage.ID, newMessage.LastUpdateAt)
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
		parentID, _ = p.store.MattermostToTeamsPostID(post.RootId)
	}

	text := post.Message
	client, err := p.getClientForUser(user.Id)
	if err != nil {
		client = p.msteamsBotClient
		text = user.Username + ":\n\n" + post.Message
	}

	var attachments []*msteams.Attachment
	for _, fileId := range post.FileIds {
		fileInfo, appErr := p.API.GetFileInfo(fileId)
		if appErr != nil {
			p.API.LogWarn("Unable to get file attachment", "error", appErr)
			continue
		}
		fileData, appErr := p.API.GetFile(fileInfo.Id)
		if appErr != nil {
			p.API.LogWarn("error get file attachment from mattermost", "error", appErr)
			continue
		}

		attachment, err := client.UploadFile(teamID, channelID, fileInfo.Id+"_"+fileInfo.Name, int(fileInfo.Size), fileInfo.MimeType, bytes.NewReader(fileData))
		if err != nil {
			p.API.LogWarn("error uploading attachment", "error", err)
			continue
		}
		attachments = append(attachments, attachment)
	}

	newMessage, err := client.SendMessageWithAttachments(teamID, channelID, parentID, text, attachments)
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err)
		return "", err
	}

	if post.Id != "" && newMessage != nil {
		err := p.store.LinkPosts(post.Id, channelID, newMessage.ID, newMessage.LastUpdateAt)
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
		parentID, _ = p.store.MattermostToTeamsPostID(post.RootId)
	}

	client, err := p.getClientForUser(user.Id)
	if err != nil {
		client = p.msteamsBotClient
	}

	msgID, _ := p.store.MattermostToTeamsPostID(post.Id)

	if err := client.DeleteMessage(teamID, channelID, parentID, msgID); err != nil {
		p.API.LogError("Error deleting post", "error", err)
		return err
	}

	return nil
}

func (p *Plugin) DeleteChat(chatID string, user *model.User, post *model.Post) error {
	p.API.LogDebug("Deleting direct message to MS Teams", "chatID", chatID, "post", post)

	client, err := p.getClientForUser(user.Id)
	if err != nil {
		client = p.msteamsBotClient
	}

	msgID, _ := p.store.MattermostToTeamsPostID(post.Id)

	if err := client.DeleteChatMessage(chatID, msgID); err != nil {
		p.API.LogError("Error deleting post", "error", err)
		return err
	}
	return nil
}

func (p *Plugin) Update(teamID, channelID string, user *model.User, newPost, oldPost *model.Post) error {
	p.API.LogDebug("Updating message to MS Teams", "teamID", teamID, "channelID", channelID, "oldPost", oldPost, "newPost", newPost)

	parentID := ""
	if oldPost.RootId != "" {
		parentID, _ = p.store.MattermostToTeamsPostID(newPost.RootId)
	}

	text := newPost.Message

	client, err := p.getClientForUser(user.Id)
	if err != nil {
		client = p.msteamsBotClient
		text = user.Username + ":\n\n" + newPost.Message
	}

	msgID, _ := p.store.MattermostToTeamsPostID(newPost.Id)

	if err := client.UpdateMessage(teamID, channelID, parentID, msgID, text); err != nil {
		p.API.LogWarn("Error updating the post", "error", err)
		return err
	}

	var updatedMessage *msteams.Message
	if parentID != "" {
		updatedMessage, err = client.GetReply(teamID, channelID, parentID, msgID)
	} else {
		updatedMessage, err = client.GetMessage(teamID, channelID, msgID)
	}
	if err != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
	} else {
		err := p.store.LinkPosts(newPost.Id, channelID, msgID, updatedMessage.LastUpdateAt)
		if err != nil {
			p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
		}
	}

	return nil
}

func (p *Plugin) UpdateChat(chatID string, user *model.User, newPost, oldPost *model.Post) error {
	p.API.LogDebug("Updating direct message to MS Teams", "chatID", chatID, "oldPost", oldPost, "newPost", newPost)

	msgID, _ := p.store.MattermostToTeamsPostID(newPost.Id)

	text := newPost.Message

	client, err := p.getClientForUser(user.Id)
	if err != nil {
		client = p.msteamsBotClient
		text = user.Username + ":\n\n" + newPost.Message
	}

	if err := client.UpdateChatMessage(chatID, msgID, text); err != nil {
		p.API.LogWarn("Error updating the post", "error", err)
		return err
	}

	updatedMessage, err := client.GetChatMessage(chatID, msgID)
	if err != nil {
		p.API.LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
	} else {
		err := p.store.LinkPosts(newPost.Id, chatID, msgID, updatedMessage.LastUpdateAt)
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
	client, err := p.getClientForUser(clientUserID)
	if err != nil {
		return "", err
	}
	chatID, err := client.CreateOrGetChatForUsers(teamsUsersIDs)
	if err != nil {
		return "", err
	}
	return chatID, nil
}
