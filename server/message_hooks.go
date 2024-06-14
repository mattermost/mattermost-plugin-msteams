package main

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func (p *Plugin) UserWillLogIn(_ *plugin.Context, user *model.User) string {
	if p.IsRemoteUser(user) && p.getConfiguration().AutomaticallyPromoteSyntheticUsers {
		*user.RemoteId = ""
		if _, appErr := p.API.UpdateUser(user); appErr != nil {
			p.API.LogWarn("Unable to promote synthetic user", "user_id", user.Id, "error", appErr.Error())
			return "Unable to promote synthetic user"
		}

		p.API.LogInfo("Promoted synthetic user", "user_id", user.Id)
	}

	return ""
}

func (p *Plugin) MessageHasBeenDeleted(_ *plugin.Context, post *model.Post) {
	if post.Props != nil {
		if _, ok := post.Props["msteams_sync_"+p.botUserID].(bool); ok {
			return
		}
	}

	if post.IsSystemMessage() {
		return
	}

	channel, appErr := p.API.GetChannel(post.ChannelId)
	if appErr != nil {
		p.API.LogWarn("Failed to get channel on message deleted", "error", appErr.Error(), "post_id", post.Id, "channel_id", post.ChannelId)
		return
	}

	if channel.IsGroupOrDirect() {
		chatShouldSync, _, _, _, err := p.ChatShouldSync(channel)
		if err != nil {
			p.API.LogWarn("Failed to determine if deleted message should sync", "channel_id", channel.Id, "post_id", post.Id, "error", err.Error())
			return
		} else if !chatShouldSync {
			return
		}

		if err := p.DeleteChat(post); err != nil {
			p.API.LogWarn("Unable to delete chat", "error", err.Error())
			return
		}
	} else {
		link, err := p.store.GetLinkByChannelID(post.ChannelId)
		if err != nil || link == nil {
			return
		}

		if !p.getConfiguration().SyncLinkedChannels {
			return
		}

		user, _ := p.API.GetUser(post.UserId)
		if err = p.Delete(link.MSTeamsTeam, link.MSTeamsChannel, user, post); err != nil {
			p.API.LogWarn("Unable to delete message", "error", err.Error())
			return
		}
	}
}

func (p *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	channel, appErr := p.API.GetChannel(post.ChannelId)
	if appErr != nil {
		return
	}

	isDirectOrGroupMessage := channel.IsGroupOrDirect()

	if post.Props != nil {
		if _, ok := post.Props["msteams_sync_"+p.botUserID].(bool); ok {
			return
		}
	}

	if post.IsSystemMessage() {
		return
	}

	if isDirectOrGroupMessage {
		chatShouldSync, containsRemoteUser, members, _, err := p.ChatShouldSync(channel)
		if err != nil {
			p.API.LogWarn("Failed to determine if posted message should sync", "channel_id", channel.Id, "error", err.Error())
			return
		} else if !chatShouldSync {
			return
		}

		dstUsers := []string{}
		for _, m := range members {
			dstUsers = append(dstUsers, m.UserId)
		}

		_, err = p.SendChat(post.UserId, dstUsers, post, containsRemoteUser)
		if err != nil {
			p.API.LogWarn("Unable to handle message sent", "error", err.Error())
		}
	} else {
		link, err := p.store.GetLinkByChannelID(post.ChannelId)
		if err != nil || link == nil {
			return
		}

		if !p.getConfiguration().SyncLinkedChannels {
			return
		}

		user, _ := p.API.GetUser(post.UserId)

		_, err = p.Send(link.MSTeamsTeam, link.MSTeamsChannel, user, post)
		if err != nil {
			p.API.LogWarn("Unable to handle message sent", "error", err.Error())
		}
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
		return
	} else if postInfo == nil {
		return
	}

	link, err := p.store.GetLinkByChannelID(reaction.ChannelId)
	if err != nil || link == nil {
		if p.GetSyncChats() {
			err = p.SetChatReaction(postInfo.MSTeamsID, reaction.UserId, reaction.ChannelId, reaction.EmojiName, updateRequired)
			if err != nil {
				p.API.LogWarn("Unable to handle message reaction set", "error", err.Error())
			}
		}
		return
	}

	if !p.getConfiguration().SyncLinkedChannels {
		return
	}

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
		return
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
		if p.GetSyncChats() {
			err = p.UnsetChatReaction(postInfo.MSTeamsID, reaction.UserId, post.ChannelId, reaction.EmojiName)
			if err != nil {
				p.API.LogWarn("Unable to handle chat message reaction unset", "error", err.Error())
			}
		}
		return
	}

	if !p.getConfiguration().SyncLinkedChannels {
		return
	}

	err = p.UnsetReaction(link.MSTeamsTeam, link.MSTeamsChannel, reaction.UserId, post, reaction.EmojiName)
	if err != nil {
		p.API.LogWarn("Unable to handle message reaction unset", "error", err.Error())
	}
}

func (p *Plugin) MessageHasBeenUpdated(c *plugin.Context, newPost, _ /*oldPost*/ *model.Post) {
	updateRequired := true
	if c.RequestId == "" {
		_, ignoreHook := p.activityHandler.IgnorePluginHooksMap.LoadAndDelete(fmt.Sprintf("post_%s", newPost.Id))
		updateRequired = !ignoreHook
	}

	client, err := p.GetClientForUser(newPost.UserId)
	if err != nil {
		client, err = p.GetClientForUser(p.botUserID)
		if err != nil {
			return
		}
	}

	user, _ := p.API.GetUser(newPost.UserId)

	link, err := p.store.GetLinkByChannelID(newPost.ChannelId)
	if err != nil || link == nil {
		channel, appErr := p.API.GetChannel(newPost.ChannelId)
		if appErr != nil {
			return
		}
		if !channel.IsGroupOrDirect() {
			return
		}

		var chatShouldSync bool
		var members []*model.ChannelMember
		chatShouldSync, _, members, _, err = p.ChatShouldSync(channel)
		if err != nil {
			p.API.LogWarn("Failed to determine if updated message should sync", "channel_id", channel.Id, "post_id", newPost.Id, "error", err.Error())
			return
		} else if !chatShouldSync {
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

	if !p.getConfiguration().SyncLinkedChannels {
		return
	}

	err = p.Update(link.MSTeamsTeam, link.MSTeamsChannel, user, newPost, updateRequired)
	if err != nil {
		p.API.LogWarn("Unable to handle message update", "error", err.Error())
	}
}
