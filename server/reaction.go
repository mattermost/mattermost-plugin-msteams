package main

import (
	"github.com/enescakir/emoji"
	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/pkg/errors"
)

func (p *Plugin) SetChatReaction(teamsMessageID, srcUser, channelID, emojiName string, updateRequired bool) error {
	srcUserID, err := p.store.MattermostToTeamsUserID(srcUser)
	if err != nil {
		return err
	}

	client, err := p.GetClientForUser(srcUser)
	if err != nil {
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
		return err
	}

	client, err := p.GetClientForUser(srcUser)
	if err != nil {
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
