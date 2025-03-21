// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

type Mention struct {
	Trigger    string
	Post       *model.Post
	Channel    *model.Channel
	PostAuthor *model.User
	User       *model.User
	Group      *model.Group
	IsDM       bool // true if the mention is a DM
}

type MentionParser struct {
	PAPI             plugin.API
	Notifications    []*Mention
	msteamsAppClient msteams.Client
}

func NewMentionParser(api plugin.API, msteamsAppClient msteams.Client) *MentionParser {
	return &MentionParser{
		PAPI:             api,
		msteamsAppClient: msteamsAppClient,
	}
}

func (p *MentionParser) ProcessPost(post *model.Post) error {
	mentions := p.extractMentionsFromPost(post)
	channel, err := p.PAPI.GetChannel(post.ChannelId)
	if err != nil {
		return err
	}

	for _, mention := range mentions {
		m := &Mention{
			Trigger: mention,
			Post:    post,
			Channel: channel,
		}

		switch mention {
		case "@here":
			fallthrough
		case "@channel":
			fallthrough
		case "@all":

		default:
			user := p.isUserMention(strings.TrimPrefix(mention, "@"))
			group := p.isGroupMention(strings.TrimPrefix(mention, "@"))
			if user != nil {
				m.User = user
			} else if group != nil {
				m.Group = group
			}
		}

		p.Notifications = append(p.Notifications, m)
	}

	if channel.Type == model.ChannelTypeDirect {
		p.processDMsNotifications(channel, post)
	}

	for _, notification := range p.Notifications {
		p.PAPI.LogInfo("Processed mention", "notification", *notification)
	}

	return nil
}

func (p *MentionParser) processDMsNotifications(channel *model.Channel, post *model.Post) {
	channelMembers, err := p.PAPI.GetChannelMembers(channel.Id, 0, 1000)
	if err != nil {
		p.PAPI.LogError("Failed to get channel members", "error", err.Error())
		return
	}

	for _, member := range channelMembers {
		if member.UserId == post.UserId {
			continue
		}

		user, err := p.PAPI.GetUser(member.UserId)
		if err != nil {
			p.PAPI.LogError("Failed to get user", "error", err.Error())
			continue
		}

		p.Notifications = append(p.Notifications, &Mention{
			Trigger: post.Message,
			Post:    post,
			Channel: channel,
			User:    user,
		})
	}
}

func (p *MentionParser) extractMentionsFromPost(post *model.Post) []string {
	// Regular expression to find mentions of the form @username
	mentionRegex := regexp.MustCompile(`@[a-zA-Z0-9._-]+`)
	return mentionRegex.FindAllString(post.Message, -1)
}

func (p *MentionParser) isUserMention(mention string) *model.User {
	user, err := p.PAPI.GetUserByUsername(mention)
	if err != nil {
		return nil
	}
	return user
}

func (p *MentionParser) isGroupMention(mention string) *model.Group {
	group, err := p.PAPI.GetGroupByName(mention)
	if err != nil {
		return nil
	}
	return group
}

func (p *MentionParser) SendNotifications() error {
	for _, mention := range p.Notifications {
		if err := p.SendNotification(mention); err != nil {
			p.PAPI.LogError("Failed to send notification", "error", err.Error())
		}
	}
	return nil
}

func (p *MentionParser) SendNotification(mention *Mention) error {
	if mention.IsDM {
		return p.sendDMNotification(mention)
	}

	if mention.User != nil {
		return p.sendUserNotification(mention)
	}

	if mention.Group != nil {
		return p.sendGroupNotification(mention)
	}

	switch mention.Trigger {
	case "@here":
		return p.sendChannelNotification(mention, true)
	case "@channel":
		fallthrough
	case "@all":
		return p.sendChannelNotification(mention, false)
	}
	return nil
}

func (p *MentionParser) sendDMNotification(mention *Mention) error {
	return p.sendUserActivityNotification(NewNotification(mention, mention.User))
}

func (p *MentionParser) sendUserNotification(mention *Mention) error {
	// Do not mention yourself
	if mention.Post.UserId == mention.User.Id {
		return nil
	}

	channelMembership, err := p.PAPI.GetChannelMember(mention.Post.ChannelId, mention.User.Id)
	if err != nil {
		return err
	}
	if channelMembership == nil {
		return nil
	}

	notification := NewNotification(mention, mention.User)

	return p.sendUserActivityNotification(notification)
}

func (p *MentionParser) sendGroupNotification(mention *Mention) error {
	userGroup, err := p.PAPI.GetGroupMemberUsers(mention.Group.Id, 0, 1000)
	if err != nil {
		return err
	}
	for _, user := range userGroup {
		// Avoid sending notifications to the user who posted the message even if it's part of the group
		if user.Id == mention.Post.UserId {
			continue
		}

		// only send notification if the user belongs to the channel the group was mentioned in
		channelMembership, err := p.PAPI.GetChannelMember(mention.Post.ChannelId, user.Id)
		if err != nil {
			return err
		}
		if channelMembership == nil {
			continue
		}

		notification := NewNotification(mention, user)
		if err := p.sendUserActivityNotification(notification); err != nil {
			p.PAPI.LogError("Failed to send user activity notification", "error", err.Error())
		}
	}

	return nil
}

func (p *MentionParser) sendChannelNotification(mention *Mention, onlineOnly bool) error {
	channelMembers, err := p.PAPI.GetChannelMembers(mention.Post.ChannelId, 0, 1000)
	if err != nil {
		return err
	}

	for _, member := range channelMembers {
		if member.UserId == mention.Post.UserId {
			continue
		}

		// If online only, skip if the user is not online in mattermost.
		// Used to match the behavior of @here in Mattermost.
		if onlineOnly {
			status, err := p.PAPI.GetUserStatus(member.UserId)
			if err != nil {
				return err
			}
			if status.Status != model.StatusOnline {
				continue
			}
		}

		user, err := p.PAPI.GetUser(member.UserId)
		if err != nil {
			return err
		}

		notification := NewNotification(mention, user)
		if err := p.sendUserActivityNotification(notification); err != nil {
			p.PAPI.LogError("Failed to send user activity notification", "error", err.Error())
		}
	}

	return nil
}

func (p *MentionParser) sendUserActivityNotification(notification *Notification) error {
	user, err := p.PAPI.GetUser(notification.User.Id)
	if err != nil {
		return err
	}

	appID, exists := user.GetProp(getUserPropKey("app_id"))
	if !exists {
		return nil
	}

	msteamsUserID, exists := user.GetProp(getUserPropKey("user_id"))
	if !exists {
		return nil
	}

	sender, err := p.PAPI.GetUser(notification.Mention.Post.UserId)
	if err != nil {
		p.PAPI.LogError("Failed to get sender", "error", err.Error())
		return err
	}

	// Extract post message to use in notification
	message := notification.Mention.Post.Message
	if len(message) > 100 {
		message = message[:97] + "..."
	}

	context := map[string]string{
		"subEntityId": fmt.Sprintf("post_%s", notification.Mention.Post.Id),
	}

	jsonContext, jsonErr := json.Marshal(context)
	if jsonErr != nil {
		p.PAPI.LogError("Failed to marshal context", "error", jsonErr.Error())
		return jsonErr
	}

	urlParams := url.Values{}
	urlParams.Set("context", string(jsonContext))

	if err := p.msteamsAppClient.SendUserActivity(msteamsUserID, "mattermost_mention_with_name", message, url.URL{
		Scheme:   "https",
		Host:     "teams.microsoft.com",
		Path:     "/l/entity/" + appID + "/" + context["subEntityId"],
		RawQuery: urlParams.Encode(),
	}, map[string]string{
		"post_author": sender.GetDisplayName(model.ShowNicknameFullName),
	}); err != nil {
		p.PAPI.LogError("Failed to send user activity notification", "error", err.Error())
	}

	return nil
}

type Notification struct {
	Mention *Mention
	User    *model.User
}

func NewNotification(mention *Mention, user *model.User) *Notification {
	return &Notification{
		Mention: mention,
		User:    user,
	}
}
