package handlers

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/enescakir/emoji"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

var emojisReverseMap map[string]string

var attachRE = regexp.MustCompile(`<attachment id=.*?attachment>`)

const (
	lastReceivedChangeKey = "last_received_change"
	numberOfWorkers       = 20
	activityQueueSize     = 1000
)

type PluginIface interface {
	GetAPI() plugin.API
	GetStore() store.Store
	GetSyncDirectMessages() bool
	GetBotUserID() string
	GetURL() string
	GetClientForApp() msteams.Client
	GetClientForUser(string) (msteams.Client, error)
	GetClientForTeamsUser(string) (msteams.Client, error)
	GenerateRandomPassword() string
}

type ActivityHandler struct {
	plugin PluginIface
	queue  chan msteams.Activity
	quit   chan bool
}

func New(plugin PluginIface) *ActivityHandler {
	// Initialize the emoji translator
	emojisReverseMap = map[string]string{}
	for alias, unicode := range emoji.Map() {
		emojisReverseMap[unicode] = strings.Replace(alias, ":", "", 2)
	}
	emojisReverseMap["like"] = "+1"
	emojisReverseMap["sad"] = "cry"
	emojisReverseMap["angry"] = "angry"
	emojisReverseMap["laugh"] = "laughing"
	emojisReverseMap["heart"] = "heart"
	emojisReverseMap["surprised"] = "open_mouth"
	emojisReverseMap["checkmarkbutton"] = "white_check_mark"

	return &ActivityHandler{
		plugin: plugin,
		queue:  make(chan msteams.Activity, activityQueueSize),
		quit:   make(chan bool),
	}
}

func (ah *ActivityHandler) Start() {
	for i := 0; i < numberOfWorkers; i++ {
		go func() {
			for {
				select {
				case activity := <-ah.queue:
					ah.handleActivity(activity)
				case <-ah.quit:
					// we have received a signal to stop
					return
				}
			}
		}()
	}
}

func (ah *ActivityHandler) Stop() {
	go func() {
		for i := 0; i < numberOfWorkers; i++ {
			ah.quit <- true
		}
	}()
}

func (ah *ActivityHandler) Handle(activity msteams.Activity) error {
	ah.queue <- activity
	return nil
}

func (ah *ActivityHandler) HandleLifecycleEvent(event msteams.Activity, webhookSecret string, evaluationAPI bool) {
	if !ah.checkSubscription(event.SubscriptionID) {
		return
	}

	if event.LifecycleEvent == "reauthorizationRequired" {
		expiresOn, err := ah.plugin.GetClientForApp().RefreshSubscription(event.SubscriptionID)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to refresh the subscription", "error", err.Error())
		} else {
			if err2 := ah.plugin.GetStore().UpdateSubscriptionExpiresOn(event.SubscriptionID, *expiresOn); err2 != nil {
				ah.plugin.GetAPI().LogError("Unable to store the subscription new expires date", "error", err2.Error())
			}
		}
	} else if event.LifecycleEvent == "subscriptionRemoved" {
		_, err := ah.plugin.GetClientForApp().SubscribeToChannels(ah.plugin.GetURL()+"/", webhookSecret, !evaluationAPI)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to subscribe to channels", "error", err)
		}

		_, err = ah.plugin.GetClientForApp().SubscribeToChats(ah.plugin.GetURL()+"/", webhookSecret, !evaluationAPI)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to subscribe to chats", "error", err)
		}
	}
}

func (ah *ActivityHandler) checkSubscription(subscriptionID string) bool {
	subscriptionType, err := ah.plugin.GetStore().GetSubscriptionType(subscriptionID)
	if err != nil {
		// Ignoring the error because can be the case that the subscription is no longer exists, in that case, it doesn't matter.
		_ = ah.plugin.GetClientForApp().DeleteSubscription(subscriptionID)
		return false
	}

	if subscriptionType == "allChats" {
		return true
	}

	switch subscriptionType {
	case "allChats":
		return true
	case "channel":
		subscription, err := ah.plugin.GetStore().GetChannelSubscription(subscriptionID)
		if err != nil {
			// Ignoring the error because can be the case that the subscription is no longer exists, in that case, it doesn't matter.
			_ = ah.plugin.GetClientForApp().DeleteSubscription(subscriptionID)
			return false
		}
		_, err = ah.plugin.GetStore().GetLinkByMSTeamsChannelID(subscription.TeamID, subscription.ChannelID)
		if err != nil {
			// Ignoring the error because can be the case that the subscription is no longer exists, in that case, it doesn't matter.
			_ = ah.plugin.GetStore().DeleteSubscription(subscriptionID)
			// Ignoring the error because can be the case that the subscription is no longer exists, in that case, it doesn't matter.
			_ = ah.plugin.GetClientForApp().DeleteSubscription(subscriptionID)
			return false
		}
	case "chat":
		subscription, err := ah.plugin.GetStore().GetChatSubscription(subscriptionID)
		if err != nil {
			// Ignoring the error because can be the case that the subscription is no longer exists, in that case, it doesn't matter.
			_ = ah.plugin.GetClientForApp().DeleteSubscription(subscriptionID)
			return false
		}
		if _, appErr := ah.plugin.GetAPI().GetUser(subscription.UserID); appErr != nil {
			// Ignoring the error because can be the case that the subscription is no longer exists, in that case, it doesn't matter.
			_ = ah.plugin.GetStore().DeleteSubscription(subscriptionID)
			// Ignoring the error because can be the case that the subscription is no longer exists, in that case, it doesn't matter.
			_ = ah.plugin.GetClientForApp().DeleteSubscription(subscriptionID)
			return false
		}
	}

	return true
}

func (ah *ActivityHandler) handleActivity(activity msteams.Activity) {
	activityIds := msteams.GetResourceIds(activity.Resource)

	if !ah.checkSubscription(activity.SubscriptionID) {
		ah.plugin.GetAPI().LogError("The subscription is no longer active", "subscriptionID", activity.SubscriptionID)
		return
	}

	switch activity.ChangeType {
	case "created":
		ah.plugin.GetAPI().LogDebug("Handling create activity", "activity", activity)
		ah.handleCreatedActivity(activityIds)
	case "updated":
		ah.plugin.GetAPI().LogDebug("Handling update activity", "activity", activity)
		ah.handleUpdatedActivity(activityIds)
	case "deleted":
		ah.plugin.GetAPI().LogDebug("Handling delete activity", "activity", activity)
		ah.handleDeletedActivity(activityIds)
	default:
		ah.plugin.GetAPI().LogWarn("Unandledy activity", "activity", activity, "error", "Not handled activity")
	}
}

func (ah *ActivityHandler) handleCreatedActivity(activityIds msteams.ActivityIds) {
	msg, chat, err := ah.getMessageAndChatFromActivityIds(activityIds)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to get original message", "error", err.Error())
		return
	}

	if msg == nil {
		ah.plugin.GetAPI().LogDebug("Unable to get the message (probably because belongs to private chat in not-linked users)")
		return
	}

	if msg.UserID == "" {
		ah.plugin.GetAPI().LogDebug("Skipping not user event", "msg", msg)
		return
	}

	msteamsUserID, _ := ah.plugin.GetStore().MattermostToTeamsUserID(ah.plugin.GetBotUserID())
	if msg.UserID == msteamsUserID {
		ah.plugin.GetAPI().LogDebug("Skipping messages from bot user")
		ah.updateLastReceivedChangeDate(msg.LastUpdateAt)
		return
	}

	var senderID string
	var channelID string
	if chat != nil {
		if !ah.plugin.GetSyncDirectMessages() {
			// Skipping because direct/group messages are disabled
			return
		}

		channelID, err = ah.getChatChannelID(chat)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to get original channel id", "error", err.Error())
			return
		}
		senderID, _ = ah.plugin.GetStore().TeamsToMattermostUserID(msg.UserID)
	} else {
		senderID, _ = ah.getOrCreateSyntheticUser(msg.UserID, "")
		channelLink, _ := ah.plugin.GetStore().GetLinkByMSTeamsChannelID(msg.TeamID, msg.ChannelID)
		if channelLink != nil {
			channelID = channelLink.MattermostChannelID
		}
	}

	if err != nil || senderID == "" {
		senderID = ah.plugin.GetBotUserID()
	}

	if channelID == "" {
		ah.plugin.GetAPI().LogDebug("Channel not set")
		return
	}

	var userID string
	if msg.TeamID != "" && msg.ChannelID != "" {
		userID = ah.getUserIDForChannelLink(msg.TeamID, msg.ChannelID)
	}

	post, err := ah.msgToPost(userID, channelID, msg, senderID)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to transform teams post in mattermost post", "message", msg, "error", err)
		return
	}

	ah.plugin.GetAPI().LogDebug("Post generated", "post", post)

	// Avoid possible duplication
	postInfo, _ := ah.plugin.GetStore().GetPostInfoByMSTeamsID(msg.ChatID+msg.ChannelID, msg.ID)
	if postInfo != nil {
		ah.plugin.GetAPI().LogDebug("duplicated post")
		ah.updateLastReceivedChangeDate(msg.LastUpdateAt)
		return
	}

	newPost, appErr := ah.plugin.GetAPI().CreatePost(post)
	if appErr != nil {
		ah.plugin.GetAPI().LogError("Unable to create post", "post", post, "error", appErr)
		return
	}

	ah.plugin.GetAPI().LogDebug("Post created", "post", newPost)

	ah.updateLastReceivedChangeDate(msg.LastUpdateAt)
	if newPost != nil && newPost.Id != "" && msg.ID != "" {
		err = ah.plugin.GetStore().LinkPosts(storemodels.PostInfo{MattermostID: newPost.Id, MSTeamsChannel: msg.ChatID + msg.ChannelID, MSTeamsID: msg.ID, MSTeamsLastUpdateAt: msg.LastUpdateAt})
		if err != nil {
			ah.plugin.GetAPI().LogWarn("Error updating the msteams/mattermost post link metadata", "error", err)
		}
	}
}

func (ah *ActivityHandler) handleUpdatedActivity(activityIds msteams.ActivityIds) {
	msg, chat, err := ah.getMessageAndChatFromActivityIds(activityIds)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to get original message", "error", err.Error())
		return
	}

	if msg == nil {
		ah.plugin.GetAPI().LogDebug("Unable to get the message (probably because belongs to private chat in not-linked users)")
		return
	}

	if msg.UserID == "" {
		ah.plugin.GetAPI().LogDebug("Skipping not user event", "msg", msg)
		return
	}

	msteamsUserID, _ := ah.plugin.GetStore().MattermostToTeamsUserID(ah.plugin.GetBotUserID())
	if msg.UserID == msteamsUserID {
		ah.plugin.GetAPI().LogDebug("Skipping messages from bot user")
		ah.updateLastReceivedChangeDate(msg.LastUpdateAt)
		return
	}

	postInfo, _ := ah.plugin.GetStore().GetPostInfoByMSTeamsID(msg.ChatID+msg.ChannelID, msg.ID)
	if postInfo == nil {
		ah.updateLastReceivedChangeDate(msg.LastUpdateAt)
		return
	}

	// Ingnore if the change is already applied in the database
	if postInfo.MSTeamsLastUpdateAt.UnixMicro() == msg.LastUpdateAt.UnixMicro() {
		return
	}

	channelID := ""
	if chat == nil {
		var channelLink *storemodels.ChannelLink
		channelLink, err = ah.plugin.GetStore().GetLinkByMSTeamsChannelID(msg.TeamID, msg.ChannelID)
		if err != nil || channelLink == nil {
			ah.plugin.GetAPI().LogError("Unable to find the subscription")
			return
		}
		channelID = channelLink.MattermostChannelID
		if !ah.plugin.GetSyncDirectMessages() {
			// Skipping because direct/group messages are disabled
			return
		}
	} else {
		post, err := ah.plugin.GetAPI().GetPost(postInfo.MattermostID)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to find the original post", "error", err.Error())
			return
		}
		channelID = post.ChannelId
	}

	senderID, err := ah.plugin.GetStore().TeamsToMattermostUserID(msg.UserID)
	if err != nil || senderID == "" {
		senderID = ah.plugin.GetBotUserID()
	}

	var userID string
	if msg.TeamID != "" && msg.ChannelID != "" {
		userID = ah.getUserIDForChannelLink(msg.TeamID, msg.ChannelID)
	}

	post, err := ah.msgToPost(userID, channelID, msg, senderID)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to transform teams post in mattermost post", "message", msg, "error", err)
		return
	}

	post.Id = postInfo.MattermostID

	_, appErr := ah.plugin.GetAPI().UpdatePost(post)
	if appErr != nil {
		ah.plugin.GetAPI().LogError("Unable to update post", "post", post, "error", appErr)
		return
	}

	ah.updateLastReceivedChangeDate(msg.LastUpdateAt)
	ah.plugin.GetAPI().LogError("Message reactions", "reactions", msg.Reactions, "error", err)
	ah.handleReactions(postInfo.MattermostID, channelID, msg.Reactions)
}

func (ah *ActivityHandler) handleReactions(postID string, channelID string, reactions []msteams.Reaction) {
	ah.plugin.GetAPI().LogDebug("Handling reactions", "reactions", reactions)

	postReactions, appErr := ah.plugin.GetAPI().GetReactions(postID)
	if appErr != nil {
		return
	}

	if len(reactions) == 0 && len(postReactions) == 0 {
		return
	}

	postReactionsByUserAndEmoji := map[string]bool{}
	for _, pr := range postReactions {
		postReactionsByUserAndEmoji[pr.UserId+pr.EmojiName] = true
	}

	allReactions := map[string]bool{}
	for _, reaction := range reactions {
		emojiName, ok := emojisReverseMap[reaction.Reaction]
		if !ok {
			ah.plugin.GetAPI().LogError("No code reaction found for reaction", "reaction", reaction.Reaction)
			continue
		}
		reactionUserID, err := ah.plugin.GetStore().TeamsToMattermostUserID(reaction.UserID)
		if err != nil {
			ah.plugin.GetAPI().LogError("unable to find the user for the reaction", "reaction", reaction.Reaction)
			continue
		}
		allReactions[reactionUserID+emojiName] = true
	}

	for _, r := range postReactions {
		if !allReactions[r.UserId+r.EmojiName] {
			r.ChannelId = "removedfromplugin"
			appErr = ah.plugin.GetAPI().RemoveReaction(r)
			if appErr != nil {
				ah.plugin.GetAPI().LogError("Unable to remove reaction", "error", appErr.Error())
			}
		}
	}

	for _, reaction := range reactions {
		reactionUserID, err := ah.plugin.GetStore().TeamsToMattermostUserID(reaction.UserID)
		if err != nil {
			ah.plugin.GetAPI().LogError("unable to find the user for the reaction", "reaction", reaction.Reaction)
			continue
		}
		emojiName, ok := emojisReverseMap[reaction.Reaction]
		if !ok {
			ah.plugin.GetAPI().LogError("No code reaction found for reaction", "reaction", reaction.Reaction)
			continue
		}
		if !postReactionsByUserAndEmoji[reactionUserID+emojiName] {
			r, appErr := ah.plugin.GetAPI().AddReaction(&model.Reaction{
				UserId:    reactionUserID,
				PostId:    postID,
				ChannelId: channelID,
				EmojiName: emojiName,
			})
			if appErr != nil {
				ah.plugin.GetAPI().LogError("failed to create the reaction", "err", appErr)
				continue
			}
			ah.plugin.GetAPI().LogError("Added reaction", "reaction", r)
		}
	}
}

func (ah *ActivityHandler) handleDeletedActivity(activityIds msteams.ActivityIds) {
	messageID := activityIds.MessageID
	if activityIds.ReplyID != "" {
		messageID = activityIds.ReplyID
	}
	postInfo, _ := ah.plugin.GetStore().GetPostInfoByMSTeamsID(activityIds.ChatID+activityIds.ChannelID, messageID)
	if postInfo == nil {
		return
	}

	appErr := ah.plugin.GetAPI().DeletePost(postInfo.MattermostID)
	if appErr != nil {
		ah.plugin.GetAPI().LogError("Unable to to delete post", "msgID", postInfo.MattermostID, "error", appErr)
		return
	}
}

func (ah *ActivityHandler) updateLastReceivedChangeDate(t time.Time) {
	err := ah.plugin.GetAPI().KVSet(lastReceivedChangeKey, []byte(strconv.FormatInt(t.UnixMicro(), 10)))
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to store properly the last received change")
	}
}
