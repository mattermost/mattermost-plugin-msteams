// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/pluginstore"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	TeamsPropertyObjectID    = "oid"
	TeamsPropertyAppID       = "app_id"
	TeamsPropertySSOUsername = "sso_username"
)

type UserNotification struct {
	Trigger    string
	Post       *model.Post
	Channel    *model.Channel
	PostAuthor *model.User
	User       *model.User
	Group      *model.Group
}

type NotificationsParser struct {
	PAPI             plugin.API
	pluginStore      *pluginstore.PluginStore
	Notifications    []*UserNotification
	msteamsAppClient msteams.Client
}

func NewNotificationsParser(api plugin.API, pluginStore *pluginstore.PluginStore, msteamsAppClient msteams.Client) *NotificationsParser {
	return &NotificationsParser{
		PAPI:             api,
		pluginStore:      pluginStore,
		msteamsAppClient: msteamsAppClient,
	}
}

func (p *NotificationsParser) ProcessPost(post *model.Post) error {
	mentions := p.extractMentionsFromPost(post)
	channel, err := p.PAPI.GetChannel(post.ChannelId)
	if err != nil {
		return err
	}

	for _, mention := range mentions {
		m := &UserNotification{
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
			if user != nil {
				m.User = user
			} else {
				group := p.isGroupMention(strings.TrimPrefix(mention, "@"))
				if group != nil {
					m.Group = group
				}
			}

			if m.User == nil && m.Group == nil {
				p.PAPI.LogDebug("Failed to find user or group for mention", "mention", mention)
				continue
			}
		}

		p.Notifications = append(p.Notifications, m)
	}

	// Handle messages in direct and group channels, since those are not mentions.
	// TODO: Avoid repeating notifications if the message contains a mention.
	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		p.Notifications = append(p.Notifications, &UserNotification{
			Trigger: post.Message,
			Post:    post,
			Channel: channel,
		})
	}

	return nil
}

func (p *NotificationsParser) extractMentionsFromPost(post *model.Post) []string {
	// Regular expression to find mentions of the form @username
	mentionRegex := regexp.MustCompile(`@[a-zA-Z0-9._-]+`)
	return mentionRegex.FindAllString(post.Message, -1)
}

func (p *NotificationsParser) isUserMention(mention string) *model.User {
	user, err := p.PAPI.GetUserByUsername(mention)
	if err != nil {
		return nil
	}
	return user
}

func (p *NotificationsParser) isGroupMention(mention string) *model.Group {
	group, err := p.PAPI.GetGroupByName(mention)
	if err != nil {
		return nil
	}
	return group
}

func (p *NotificationsParser) SendNotifications() error {
	for _, userNotification := range p.Notifications {
		if err := p.SendNotification(userNotification); err != nil {
			p.PAPI.LogError("Failed to send notification", "error", err.Error())
		}
	}
	return nil
}

func (p *NotificationsParser) SendNotification(notification *UserNotification) error {
	// Send notifications for direct and group messages
	if notification.Channel.Type == model.ChannelTypeDirect || notification.Channel.Type == model.ChannelTypeGroup {
		return p.sendChannelNotification(notification, false)
	}

	if notification.User != nil {
		return p.sendUserNotification(notification)
	}

	if notification.Group != nil {
		return p.sendGroupNotification(notification)
	}

	switch notification.Trigger {
	case "@here":
		return p.sendChannelNotification(notification, true)
	case "@all", "@channel":
		return p.sendChannelNotification(notification, false)
	}
	return nil
}

func (p *NotificationsParser) sendUserNotification(un *UserNotification) error {
	// Do not mention yourself
	if un.Post.UserId == un.User.Id {
		return nil
	}

	channelMembership, err := p.PAPI.GetChannelMember(un.Post.ChannelId, un.User.Id)
	if err != nil {
		return err
	}
	if channelMembership == nil {
		return nil
	}

	userActivity := NewUserActivity(un, []*model.User{un.User})

	return p.sendUserActivity(userActivity)
}

func (p *NotificationsParser) sendGroupNotification(un *UserNotification) error {
	userGroup, err := p.PAPI.GetGroupMemberUsers(un.Group.Id, 0, 1000)
	if err != nil {
		return err
	}

	users := []*model.User{}
	for _, user := range userGroup {
		// Avoid sending notifications to the user who posted the message even if it's part of the group
		if user.Id == un.Post.UserId {
			continue
		}

		// only send notification if the user belongs to the channel the group was mentioned in
		channelMembership, err := p.PAPI.GetChannelMember(un.Post.ChannelId, user.Id)
		if err != nil {
			return err
		}
		if channelMembership == nil {
			continue
		}

		users = append(users, user)
	}

	userActivity := NewUserActivity(un, users)
	return p.sendUserActivity(userActivity)
}

func (p *NotificationsParser) sendChannelNotification(un *UserNotification, onlineOnly bool) error {
	channelMembers, err := p.PAPI.GetChannelMembers(un.Post.ChannelId, 0, 1000)
	if err != nil {
		return err
	}

	users := []*model.User{}
	for _, member := range channelMembers {
		// Avoid sending notifications to the user who posted the message even if it's part of the channel
		if member.UserId == un.Post.UserId {
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

		users = append(users, user)
	}

	userActivity := NewUserActivity(un, users)
	return p.sendUserActivity(userActivity)
}

func (p *NotificationsParser) sendUserActivity(userActivity *UserActivity) error {
	sender, appErr := p.PAPI.GetUser(userActivity.UserNotification.Post.UserId)
	if appErr != nil {
		p.PAPI.LogError("Failed to get sender", "error", appErr.Error())
		return appErr
	}

	// Extract post message to use in notification
	message := userActivity.UserNotification.Post.Message
	if len(message) > 100 {
		message = message[:97] + "..."
	}

	context := map[string]string{
		"subEntityId": fmt.Sprintf("post_preview_%s", userActivity.UserNotification.Post.Id),
	}

	jsonContext, jsonErr := json.Marshal(context)
	if jsonErr != nil {
		p.PAPI.LogError("Failed to marshal context", "error", jsonErr.Error())
		return jsonErr
	}

	urlParams := url.Values{}
	urlParams.Set("context", string(jsonContext))

	appID, err := p.pluginStore.GetAppID()
	if err != nil {
		p.PAPI.LogError("Failed to get app ID", "error", err.Error())
		return fmt.Errorf("failed to get app ID: %w", err)
	}

	msteamsUserIDs := []string{}
	for _, user := range userActivity.Users {
		storedUser, err := p.pluginStore.GetUser(user.Id)
		if err != nil {
			p.PAPI.LogError("Failed to get stored user", "error", err.Error())
			continue
		}

		msteamsUserIDs = append(msteamsUserIDs, storedUser.TeamsObjectID)
	}

	if len(msteamsUserIDs) == 0 {
		p.PAPI.LogDebug("No users to notify")
		return nil
	}

	if err := p.msteamsAppClient.SendUserActivity(msteamsUserIDs, "mattermost_mention_with_name", message, url.URL{
		Scheme:   "https",
		Host:     "teams.microsoft.com",
		Path:     "/l/entity/" + appID + "/notification_preview",
		RawQuery: urlParams.Encode(),
	}, map[string]string{
		"post_author": sender.GetDisplayName(model.ShowNicknameFullName),
	}); err != nil {
		p.PAPI.LogError("Failed to send user activity notification", "error", err.Error())
	}

	return nil
}

type UserActivity struct {
	UserNotification *UserNotification
	Users            []*model.User
}

func NewUserActivity(mention *UserNotification, users []*model.User) *UserActivity {
	return &UserActivity{
		UserNotification: mention,
		Users:            users,
	}
}
