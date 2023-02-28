package main

import (
	"bytes"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

func (p *Plugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
	if len(post.FileIds) > 0 {
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
				if strings.HasSuffix(user.Email, "@msteamssync-plugin") {
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
		if channel.Type == model.ChannelTypeDirect {
			members, err := p.API.GetChannelMembers(post.ChannelId, 0, 2)
			if err != nil {
				return
			}
			var dstUser string
			for _, m := range members {
				if m.UserId != post.UserId {
					dstUser = m.UserId
				}
			}
			p.SendChat(dstUser, post.UserId, post)
		}
		if channel.Type == model.ChannelTypeGroup {
			// TODO: Add support for group messages
			panic("Fix this for group messages")
		}
		return
	}

	user, _ := p.API.GetUser(post.UserId)

	p.Send(link.MSTeamsTeam, link.MSTeamsChannel, user, post)
}

func (p *Plugin) MessageHasBeenUpdated(c *plugin.Context, newPost, oldPost *model.Post) {
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
		members, appErr := p.API.GetChannelMembers(newPost.ChannelId, 0, 2)
		if appErr != nil {
			return
		}
		if len(members) != 2 {
			return
		}
		dstUserID, err := p.store.MattermostToTeamsUserId(members[0].UserId)
		if err != nil {
			return
		}
		srcUserID, err := p.store.MattermostToTeamsUserId(members[1].UserId)
		if err != nil {
			return
		}
		chatID, err := client.CreateOrGetChatForUsers(dstUserID, srcUserID)
		if err != nil {
			return
		}
		p.UpdateChat(chatID, user, newPost, oldPost)
		return
	}

	p.Update(link.MSTeamsTeam, link.MSTeamsChannel, user, newPost, oldPost)
}

func (p *Plugin) SendChat(dstUser, srcUser string, post *model.Post) (string, error) {
	p.API.LogDebug("Sending direct message to MS Teams", "srcUser", srcUser, "dstUser", dstUser, "post", post)

	parentID := ""
	if post.RootId != "" {
		parentID, _ = p.store.MattermostToTeamsPostId(post.RootId)
	}

	dstUserID, err := p.store.MattermostToTeamsUserId(dstUser)
	if err != nil {
		return "", err
	}
	srcUserID, err := p.store.MattermostToTeamsUserId(dstUser)
	if err != nil {
		return "", err
	}

	p.API.LogDebug("Sending direct message to MS Teams", "srcUserID", srcUserID, "dstUserID", dstUserID, "post", post)
	text := post.Message

	client, err := p.getClientForUser(srcUser)
	if err != nil {
		return "", err
	}

	chatID, err := client.CreateOrGetChatForUsers(dstUserID, srcUserID)
	if err != nil {
		p.API.LogError("FAILING TO CREATE OR GET THE CHAT", "error", err)
		return "", err
	}

	newMessageId, err := client.SendChat(chatID, parentID, text)
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err)
		return "", err
	}

	if post.Id != "" && newMessageId != "" {
		p.store.LinkPosts(post.Id, chatID, newMessageId)
	}
	return newMessageId, nil
}

func (p *Plugin) Send(teamID, channelID string, user *model.User, post *model.Post) (string, error) {
	p.API.LogDebug("Sending message to MS Teams", "teamID", teamID, "channelID", channelID, "post", post)

	parentID := ""
	if post.RootId != "" {
		parentID, _ = p.store.MattermostToTeamsPostId(post.RootId)
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

	newMessageId, err := client.SendMessageWithAttachments(teamID, channelID, parentID, text, attachments)
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err)
		return "", err
	}

	if post.Id != "" && newMessageId != "" {
		p.store.LinkPosts(post.Id, channelID, newMessageId)
	}
	return newMessageId, nil
}

func (p *Plugin) Delete(teamID, channelID string, user *model.User, post *model.Post) error {
	p.API.LogDebug("Sending message to MS Teams", "teamID", teamID, "channelID", channelID, "post", post)

	parentID := ""
	if post.RootId != "" {
		parentID, _ = p.store.MattermostToTeamsPostId(post.RootId)
	}

	client, err := p.getClientForUser(user.Id)
	if err != nil {
		client = p.msteamsBotClient
	}

	msgID, _ := p.store.MattermostToTeamsPostId(post.Id)

	if err := client.DeleteMessage(teamID, channelID, parentID, msgID); err != nil {
		p.API.LogError("Error deleting post", "error", err)
		return err
	}

	return nil
}

func (p *Plugin) DeleteChat(chatID string, user *model.User, post *model.Post) error {
	p.API.LogDebug("Sending message to MS Teams", "chatID", chatID, "post", post)

	client, err := p.getClientForUser(user.Id)
	if err != nil {
		client = p.msteamsBotClient
	}

	msgID, _ := p.store.MattermostToTeamsPostId(post.Id)

	if err := client.DeleteChatMessage(chatID, msgID); err != nil {
		p.API.LogError("Error deleting post", "error", err)
		return err
	}
	return nil
}

func (p *Plugin) Update(teamID, channelID string, user *model.User, newPost, oldPost *model.Post) error {
	p.API.LogDebug("Sending message to MS Teams", "teamID", teamID, "channelID", channelID, "oldPost", oldPost, "newPost", newPost)

	parentID := ""
	if oldPost.RootId != "" {
		parentID, _ = p.store.MattermostToTeamsPostId(newPost.RootId)
	}

	text := newPost.Message

	client, err := p.getClientForUser(user.Id)
	if err != nil {
		client = p.msteamsBotClient
		text = user.Username + ":\n\n" + newPost.Message
	}

	msgID, _ := p.store.MattermostToTeamsPostId(newPost.Id)

	if err := client.UpdateMessage(teamID, channelID, parentID, msgID, text); err != nil {
		p.API.LogWarn("Error updating the post", "error", err)
		return err
	}

	return nil
}

func (p *Plugin) UpdateChat(chatID string, user *model.User, newPost, oldPost *model.Post) error {
	p.API.LogDebug("Sending message to MS Teams", "chatID", chatID, "oldPost", oldPost, "newPost", newPost)

	msgID, _ := p.store.MattermostToTeamsPostId(newPost.Id)

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

	return nil
}
