// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/markdown"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

const (
	// Different types of mentions ordered by their priority from lowest to highest

	// A placeholder that should never be used in practice
	NoMention MentionType = iota

	// The post is in a GM
	GMMention

	// The post is in a thread that the user has commented on
	ThreadMention

	// The post is a comment on a thread started by the user
	CommentMention

	// The post contains an at-channel, at-all, or at-here
	ChannelMention

	// The post is a DM
	DMMention

	// The post contains an at-mention for the user
	KeywordMention

	// The post contains a group mention for the user
	GroupMention
)

type Notification struct {
	Post       *model.Post
	PostAuthor *model.User
	User       *model.User
	Type       MentionType
}

func NewNotification(post *model.Post, postAuthor *model.User, user *model.User, mentionType MentionType) *Notification {
	return &Notification{
		Post:       post,
		PostAuthor: postAuthor,
		User:       user,
		Type:       mentionType,
	}
}

type MentionType int

type MentionResults struct {
	Team   *model.Team
	Sender *model.User

	// Mentions maps the ID of each user that was mentioned to how they were mentioned.
	Mentions map[string]MentionType

	// GroupMentions maps the ID of each group that was mentioned to how it was mentioned.
	GroupMentions map[string]MentionType

	// OtherPotentialMentions contains a list of strings that looked like mentions, but didn't have
	// a corresponding keyword.
	OtherPotentialMentions []string

	// HereMentioned is true if the message contained @here.
	HereMentioned bool

	// AllMentioned is true if the message contained @all.
	AllMentioned bool

	// ChannelMentioned is true if the message contained @channel.
	ChannelMentioned bool
}

func (m *MentionResults) isUserMentioned(userID string) bool {
	if _, ok := m.Mentions[userID]; ok {
		return true
	}

	if _, ok := m.GroupMentions[userID]; ok {
		return true
	}

	return m.HereMentioned || m.AllMentioned || m.ChannelMentioned
}

func (m *MentionResults) addMention(userID string, mentionType MentionType) {
	if m.Mentions == nil {
		m.Mentions = make(map[string]MentionType)
	}

	if currentType, ok := m.Mentions[userID]; ok && currentType >= mentionType {
		return
	}

	m.Mentions[userID] = mentionType
}

func (m *MentionResults) removeMention(userID string) {
	delete(m.Mentions, userID)
}

func (m *MentionResults) addGroupMention(groupID string) {
	if m.GroupMentions == nil {
		m.GroupMentions = make(map[string]MentionType)
	}

	m.GroupMentions[groupID] = GroupMention
}

// Given a message and a map mapping mention keywords to the users who use them, returns a map of mentioned
// users and a slice of potential mention users not in the channel and whether or not @here was mentioned.
func getExplicitMentions(post *model.Post, keywords MentionKeywords) *MentionResults {
	parser := makeStandardMentionParser(keywords)

	buf := ""
	mentionsEnabledFields := getMentionsEnabledFields(post)
	for _, message := range mentionsEnabledFields {
		// Parse the text as Markdown, combining adjacent Text nodes into a single string for processing
		markdown.Inspect(message, func(node any) bool {
			text, ok := node.(*markdown.Text)
			if !ok {
				// This node isn't a string so process any accumulated text in the buffer
				if buf != "" {
					parser.ProcessText(buf)
				}

				buf = ""
				return true
			}

			// This node is a string, so add it to buf and continue onto the next node to see if it's more text
			buf += text.Text
			return false
		})
	}

	// Process any left over text
	if buf != "" {
		parser.ProcessText(buf)
	}

	return parser.Results()
}

// Have the compiler confirm *StandardMentionParser implements MentionParser
var _ MentionParser = &StandardMentionParser{}

type StandardMentionParser struct {
	keywords MentionKeywords

	results *MentionResults
}

func makeStandardMentionParser(keywords MentionKeywords) *StandardMentionParser {
	return &StandardMentionParser{
		keywords: keywords,

		results: &MentionResults{},
	}
}

// Processes text to filter mentioned users and other potential mentions
func (p *StandardMentionParser) ProcessText(text string) {
	systemMentions := map[string]bool{"@here": true, "@channel": true, "@all": true}

	for _, word := range strings.FieldsFunc(text, func(c rune) bool {
		// Split on any whitespace or punctuation that can't be part of an at mention or emoji pattern
		return !(c == ':' || c == '.' || c == '-' || c == '_' || c == '@' || unicode.IsLetter(c) || unicode.IsNumber(c))
	}) {
		// skip word with format ':word:' with an assumption that it is an emoji format only
		if word[0] == ':' && word[len(word)-1] == ':' {
			continue
		}

		word = strings.TrimLeft(word, ":.-_")

		if p.checkForMention(word) {
			continue
		}

		foundWithoutSuffix := false
		wordWithoutSuffix := word

		for wordWithoutSuffix != "" && strings.LastIndexAny(wordWithoutSuffix, ".-:_") == (len(wordWithoutSuffix)-1) {
			wordWithoutSuffix = wordWithoutSuffix[0 : len(wordWithoutSuffix)-1]

			if p.checkForMention(wordWithoutSuffix) {
				foundWithoutSuffix = true
				break
			}
		}

		if foundWithoutSuffix {
			continue
		}

		if _, ok := systemMentions[word]; !ok && strings.HasPrefix(word, "@") {
			// No need to bother about unicode as we are looking for ASCII characters.
			last := word[len(word)-1]
			switch last {
			// If the word is possibly at the end of a sentence, remove that character.
			case '.', '-', ':':
				word = word[:len(word)-1]
			}
			p.results.OtherPotentialMentions = append(p.results.OtherPotentialMentions, word[1:])
		} else if strings.ContainsAny(word, ".-:") {
			// This word contains a character that may be the end of a sentence, so split further
			splitWords := strings.FieldsFunc(word, func(c rune) bool {
				return c == '.' || c == '-' || c == ':'
			})

			for _, splitWord := range splitWords {
				if p.checkForMention(splitWord) {
					continue
				}
				if _, ok := systemMentions[splitWord]; !ok && strings.HasPrefix(splitWord, "@") {
					p.results.OtherPotentialMentions = append(p.results.OtherPotentialMentions, splitWord[1:])
				}
			}
		}

		if ids, match := isKeywordMultibyte(p.keywords, word); match {
			p.addMentions(ids, KeywordMention)
		}
	}
}

func (p *StandardMentionParser) Results() *MentionResults {
	return p.results
}

// checkForMention checks if there is a mention to a specific user or to the keywords here / channel / all
func (p *StandardMentionParser) checkForMention(word string) bool {
	var mentionType MentionType

	switch strings.ToLower(word) {
	case "@here":
		p.results.HereMentioned = true
		mentionType = ChannelMention
	case "@channel":
		p.results.ChannelMentioned = true
		mentionType = ChannelMention
	case "@all":
		p.results.AllMentioned = true
		mentionType = ChannelMention
	default:
		mentionType = KeywordMention
	}

	if ids, match := p.keywords[strings.ToLower(word)]; match {
		p.addMentions(ids, mentionType)
		return true
	}

	// Case-sensitive check for first name
	if ids, match := p.keywords[word]; match {
		p.addMentions(ids, mentionType)
		return true
	}

	return false
}

func (p *StandardMentionParser) addMentions(ids []MentionableID, mentionType MentionType) {
	for _, id := range ids {
		if userID, ok := id.AsUserID(); ok {
			p.results.addMention(userID, mentionType)
		} else if groupID, ok := id.AsGroupID(); ok {
			p.results.addGroupMention(groupID)
		}
	}
}

// isKeywordMultibyte checks if a word containing a multibyte character contains a multibyte keyword
func isKeywordMultibyte(keywords MentionKeywords, word string) ([]MentionableID, bool) {
	ids := []MentionableID{}
	match := false
	var multibyteKeywords []string
	for keyword := range keywords {
		if len(keyword) != utf8.RuneCountInString(keyword) {
			multibyteKeywords = append(multibyteKeywords, keyword)
		}
	}

	if len(word) != utf8.RuneCountInString(word) {
		for _, key := range multibyteKeywords {
			if strings.Contains(word, key) {
				ids, match = keywords[key]
			}
		}
	}
	return ids, match
}

const (
	mentionableUserPrefix  = "user:"
	mentionableGroupPrefix = "group:"
)

// A MentionableID stores the ID of a single User/Group with information about which type of object it refers to.
type MentionableID string

func mentionableUserID(userID string) MentionableID {
	return MentionableID(fmt.Sprint(mentionableUserPrefix, userID))
}

func mentionableGroupID(groupID string) MentionableID {
	return MentionableID(fmt.Sprint(mentionableGroupPrefix, groupID))
}

func (id MentionableID) AsUserID() (userID string, ok bool) {
	idString := string(id)
	if !strings.HasPrefix(idString, mentionableUserPrefix) {
		return "", false
	}

	return idString[len(mentionableUserPrefix):], true
}

func (id MentionableID) AsGroupID() (groupID string, ok bool) {
	idString := string(id)
	if !strings.HasPrefix(idString, mentionableGroupPrefix) {
		return "", false
	}

	return idString[len(mentionableGroupPrefix):], true
}

// MentionKeywords is a collection of mention keywords and the IDs of the objects that have a given keyword.
type MentionKeywords map[string][]MentionableID

func (k MentionKeywords) AddUser(profile *model.User, channelNotifyProps map[string]string, status *model.Status, allowChannelMentions bool) MentionKeywords {
	mentionableID := mentionableUserID(profile.Id)

	userMention := "@" + strings.ToLower(profile.Username)
	k[userMention] = append(k[userMention], mentionableID)

	// Add all the user's mention keys
	for _, mentionKey := range profile.GetMentionKeys() {
		if mentionKey != "" {
			// Note that these are made lower case so that we can do a case insensitive check for them
			mentionKey = strings.ToLower(mentionKey)

			k[mentionKey] = append(k[mentionKey], mentionableID)
		}
	}

	// If turned on, add the user's case sensitive first name
	if profile.NotifyProps[model.FirstNameNotifyProp] == "true" && profile.FirstName != "" {
		k[profile.FirstName] = append(k[profile.FirstName], mentionableID)
	}

	// Add @channel and @all to k if user has them turned on and the server allows them
	if allowChannelMentions {
		// Ignore channel mentions if channel is muted and channel mention setting is default
		ignoreChannelMentions := channelNotifyProps[model.IgnoreChannelMentionsNotifyProp] == model.IgnoreChannelMentionsOn || (channelNotifyProps[model.MarkUnreadNotifyProp] == model.UserNotifyMention && channelNotifyProps[model.IgnoreChannelMentionsNotifyProp] == model.IgnoreChannelMentionsDefault)

		if profile.NotifyProps[model.ChannelMentionsNotifyProp] == "true" && !ignoreChannelMentions {
			k["@channel"] = append(k["@channel"], mentionableID)
			k["@all"] = append(k["@all"], mentionableID)

			if status != nil && status.Status == model.StatusOnline {
				k["@here"] = append(k["@here"], mentionableID)
			}
		}
	}

	return k
}

func (k MentionKeywords) AddUserKeyword(userID string, keyword string) MentionKeywords {
	k[keyword] = append(k[keyword], mentionableUserID(userID))

	return k
}

func (k MentionKeywords) AddGroup(group *model.Group) MentionKeywords {
	if group.Name != nil {
		keyword := "@" + *group.Name
		k[keyword] = append(k[keyword], mentionableGroupID(group.Id))
	}

	return k
}

func (k MentionKeywords) AddGroupsMap(groups map[string]*model.Group) MentionKeywords {
	for _, group := range groups {
		k.AddGroup(group)
	}

	return k
}

type MentionParser interface {
	ProcessText(text string)
	Results() *MentionResults
}

// Given a post returns the values of the fields in which mentions are possible.
// post.message, preText and text in the attachment are enabled.
func getMentionsEnabledFields(post *model.Post) model.StringArray {
	ret := []string{}

	ret = append(ret, post.Message)
	for _, attachment := range post.Attachments() {
		if attachment.Pretext != "" {
			ret = append(ret, attachment.Pretext)
		}
		if attachment.Text != "" {
			ret = append(ret, attachment.Text)
		}

		for _, field := range attachment.Fields {
			if valueString, ok := field.Value.(string); ok && valueString != "" {
				ret = append(ret, valueString)
			}
		}
	}
	return ret
}

func (p *Plugin) IsCRTEnabledForUser(userID string) bool {
	appCRT := *p.API.GetConfig().ServiceSettings.CollapsedThreads
	if appCRT == model.CollapsedThreadsDisabled {
		return false
	}
	if appCRT == model.CollapsedThreadsAlwaysOn {
		return true
	}
	threadsEnabled := appCRT == model.CollapsedThreadsDefaultOn
	// check if a participant has overridden collapsed threads settings
	if preference, err := p.API.GetPreferenceForUser(userID, model.PreferenceCategoryDisplaySettings, model.PreferenceNameCollapsedThreadsEnabled); err == nil {
		threadsEnabled = preference.Value == "on"
	}
	return threadsEnabled
}

// allowChannelMentions returns whether or not the channel mentions are allowed for the given post.
func (p *Plugin) allowChannelMentions(post *model.Post, numProfiles int) bool {
	if !p.API.HasPermissionToChannel(post.UserId, post.ChannelId, model.PermissionUseChannelMentions) {
		return false
	}

	if post.Type == model.PostTypeHeaderChange || post.Type == model.PostTypePurposeChange {
		return false
	}

	if int64(numProfiles) >= *p.API.GetConfig().TeamSettings.MaxNotificationsPerChannel {
		return false
	}

	return true
}

func (p *Plugin) getMentionKeywordsInChannel(profiles map[string]*model.User, allowChannelMentions bool, channelMemberNotifyPropsMap map[string]model.StringMap, groups map[string]*model.Group) MentionKeywords {
	keywords := make(MentionKeywords)

	for _, profile := range profiles {
		status, _ := p.API.GetUserStatus(profile.Id)
		keywords.AddUser(
			profile,
			channelMemberNotifyPropsMap[profile.Id],
			status,
			allowChannelMentions,
		)
	}

	keywords.AddGroupsMap(groups)

	return keywords
}

func (p *Plugin) getExplicitMentionsAndKeywords(post *model.Post, channel *model.Channel, profileMap map[string]*model.User, groups map[string]*model.Group, channelMemberNotifyPropsMap map[string]model.StringMap, parentPostList *model.PostList) (*MentionResults, MentionKeywords) {
	mentions := &MentionResults{}
	var isAllowChannelMentions bool
	var keywords MentionKeywords

	if channel.Type == model.ChannelTypeDirect {
		isWebhook := post.GetProp("from_webhook") == "true"

		// A bot can post in a DM where it doesn't belong to.
		// Therefore, we cannot "guess" who is the other user,
		// so we add the mention to any user that is not the
		// poster unless the post comes from a webhook.
		user1, user2 := channel.GetBothUsersForDM()
		if (post.UserId != user1) || isWebhook {
			if _, ok := profileMap[user1]; ok {
				mentions.addMention(user1, DMMention)
			} else {
				p.API.LogDebug("missing profile: DM user not in profiles", mlog.String("userId", user1), mlog.String("channelId", channel.Id))
			}
		}

		if user2 != "" {
			if (post.UserId != user2) || isWebhook {
				if _, ok := profileMap[user2]; ok {
					mentions.addMention(user2, DMMention)
				} else {
					p.API.LogDebug("missing profile: DM user not in profiles", mlog.String("userId", user2), mlog.String("channelId", channel.Id))
				}
			}
		}
	} else {
		isAllowChannelMentions = p.allowChannelMentions(post, len(profileMap))
		keywords = p.getMentionKeywordsInChannel(profileMap, isAllowChannelMentions, channelMemberNotifyPropsMap, groups)

		mentions = getExplicitMentions(post, keywords)

		// Add a GM mention to all members of a GM channel
		if channel.Type == model.ChannelTypeGroup {
			for id := range channelMemberNotifyPropsMap {
				if _, ok := profileMap[id]; ok {
					mentions.addMention(id, GMMention)
				} else {
					p.API.LogDebug("missing profile: GM user not in profiles", mlog.String("userId", id), mlog.String("channelId", channel.Id))
				}
			}
		}

		// Add an implicit mention when a user is added to a channel
		// even if the user has set 'username mentions' to false in account settings.
		if post.Type == model.PostTypeAddToChannel {
			if addedUserId, ok := post.GetProp(model.PostPropsAddedUserId).(string); ok {
				if _, ok := profileMap[addedUserId]; ok {
					mentions.addMention(addedUserId, KeywordMention)
				} else {
					p.API.LogDebug("missing profile: user added to channel not in profiles", mlog.String("userId", addedUserId), mlog.String("channelId", channel.Id))
				}
			}
		}

		// Get users that have comment thread mentions enabled
		if post.RootId != "" && parentPostList != nil {
			for _, threadPost := range parentPostList.Posts {
				profile := profileMap[threadPost.UserId]
				if profile == nil {
					// Not logging missing profile since this is relatively expected
					continue
				}

				// If this is the root post and it was posted by an OAuth bot, don't notify the user
				if threadPost.Id == parentPostList.Order[0] && threadPost.IsFromOAuthBot() {
					continue
				}
				if p.IsCRTEnabledForUser(profile.Id) {
					continue
				}
				if profile.NotifyProps[model.CommentsNotifyProp] == model.CommentsNotifyAny || (profile.NotifyProps[model.CommentsNotifyProp] == model.CommentsNotifyRoot && threadPost.Id == parentPostList.Order[0]) {
					mentionType := ThreadMention
					if threadPost.Id == parentPostList.Order[0] {
						mentionType = CommentMention
					}

					mentions.addMention(threadPost.UserId, mentionType)
				}
			}
		}

		// Prevent the user from mentioning themselves
		if post.GetProp("from_webhook") != "true" {
			mentions.removeMention(post.UserId)
		}
	}

	return mentions, keywords
}

func (p *Plugin) GetAllMentions(post *model.Post) (*MentionResults, error) {
	// Get channel info
	channel, err := p.API.GetChannel(post.ChannelId)
	if err != nil {
		p.API.LogError("Failed to get channel", "error", err.Error())
		return nil, err
	}

	// Get user profiles for the channel
	profiles, err := p.API.GetUsersInChannel(post.ChannelId, "username", 0, 1000)
	if err != nil {
		p.API.LogError("Failed to get users in channel", "error", err.Error())
		return nil, err
	}

	// Convert profiles slice to map for easier lookup
	profileMap := make(map[string]*model.User)
	for _, profile := range profiles {
		profileMap[profile.Id] = profile
	}

	// Get channel member notify properties
	channelMemberNotifyPropsMap := make(map[string]model.StringMap)
	members, err := p.API.GetChannelMembers(post.ChannelId, 0, 1000)
	if err != nil {
		p.API.LogError("Failed to get channel members", "error", err.Error())
		return nil, err
	}

	for _, member := range members {
		channelMemberNotifyPropsMap[member.UserId] = member.NotifyProps
	}

	// Get any parent posts if this is a reply
	var parentPostList *model.PostList
	if post.RootId != "" {
		parentPostList, err = p.API.GetPostThread(post.RootId)
		if err != nil {
			p.API.LogError("Failed to get parent post list", "error", err.Error())
			return nil, err
		}
	}

	// Get groups (for group mentions)
	// TODO: Implement a method to get groups from the channel or the team
	// We're using an empty map for groups for simplicity
	groupsMap := make(map[string]*model.Group)

	// Extract mentions using the existing method
	mentions, _ := p.getExplicitMentionsAndKeywords(post, channel, profileMap, groupsMap, channelMemberNotifyPropsMap, parentPostList)

	return mentions, nil
}

// formatMentionsForLog converts a map of mentions to a comma-separated string for logging
func formatMentionsForLog(mentions map[string]MentionType) string {
	if len(mentions) == 0 {
		return "none"
	}

	result := "["
	first := true
	for id, mentionType := range mentions {
		if !first {
			result += ", "
		}
		result += id + ":" + formatMentionType(mentionType)
		first = false
	}
	return result + "]"
}

// formatMentionType converts a MentionType to a string representation
func formatMentionType(mentionType MentionType) string {
	switch mentionType {
	case NoMention:
		return "none"
	case GMMention:
		return "gm"
	case ThreadMention:
		return "thread"
	case CommentMention:
		return "comment"
	case ChannelMention:
		return "channel"
	case DMMention:
		return "dm"
	case KeywordMention:
		return "keyword"
	case GroupMention:
		return "group"
	default:
		return "unknown"
	}
}
