//go:generate mockery --name=PluginIface

package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enescakir/emoji"
	m "github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

var emojisReverseMap map[string]string

var attachRE = regexp.MustCompile(`<attachment id=.*?attachment>`)
var imageRE = regexp.MustCompile(`<img .*?>`)

const (
	lastReceivedChangeKey       = "last_received_change"
	numberOfWorkers             = 50
	activityQueueSize           = 5000
	msteamsUserTypeGuest        = "Guest"
	maxFileAttachmentsSupported = 10

	discardedReasonNone                   = ""
	discardedReasonInvalidChangeType      = "invalid_change_type"
	discardedReasonIsBotUser              = "is_bot_user"
	discardedReasonUnableToGetTeamsData   = "unable_to_get_teams_data"
	discardedReasonNotUserEvent           = "no_user_event"
	discardedReasonOther                  = "other"
	discardedReasonDirectMessagesDisabled = "direct_messages_disabled"
	discardedReasonInactiveUser           = "inactive_user"
	discardedReasonDuplicatedPost         = "duplicated_post"
	discardedReasonAlreadyAppliedChange   = "already_applied_change"
	discardedReasonFileLimitReached       = "file_limit_reached"
	discardedReasonEmptyFileID            = "empty_file_id"
	discardedReasonMaxFileSizeExceeded    = "max_file_size_exceeded"
	discardedReasonExpiredSubscription    = "expired_subscription"
)

type PluginIface interface {
	GetAPI() plugin.API
	GetStore() store.Store
	GetMetrics() m.Metrics
	GetSyncDirectMessages() bool
	GetSyncGuestUsers() bool
	GetMaxSizeForCompleteDownload() int
	GetBufferSizeForStreaming() int
	GetBotUserID() string
	GetURL() string
	GetClientForApp() msteams.Client
	GetClientForUser(string) (msteams.Client, error)
	GetClientForTeamsUser(string) (msteams.Client, error)
	GenerateRandomPassword() string
}

type ActivityHandler struct {
	plugin               PluginIface
	queue                chan msteams.Activity
	quit                 chan bool
	IgnorePluginHooksMap sync.Map
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
	metrics := ah.plugin.GetMetrics()
	if metrics != nil {
		// This is constant for now, but report it as a metric to future proof dashboards.
		metrics.ObserveChangeEventQueueCapacity(activityQueueSize)
	}

	for i := 0; i < numberOfWorkers; i++ {
		go func() {
			for {
				select {
				case activity := <-ah.queue:
					if metrics != nil {
						metrics.DecrementChangeEventQueueLength(activity.ChangeType)
					}
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
	metrics := ah.plugin.GetMetrics()
	if metrics != nil {
		metrics.IncrementChangeEventQueueLength(activity.ChangeType)
	}

	return nil
}

func (ah *ActivityHandler) HandleLifecycleEvent(event msteams.Activity) {
	if !ah.checkSubscription(event.SubscriptionID) {
		return
	}

	if event.LifecycleEvent == "reauthorizationRequired" {
		expiresOn, err := ah.plugin.GetClientForApp().RefreshSubscription(event.SubscriptionID)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to refresh the subscription", "error", err.Error())
		} else {
			metrics := ah.plugin.GetMetrics()
			if metrics != nil {
				metrics.ObserveSubscriptionsCount(m.SubscriptionRefreshed)
			}
			if err = ah.plugin.GetStore().UpdateSubscriptionExpiresOn(event.SubscriptionID, *expiresOn); err != nil {
				ah.plugin.GetAPI().LogError("Unable to store the subscription new expiry date", "subscriptionID", event.SubscriptionID, "error", err.Error())
			}
		}
	}
}

func (ah *ActivityHandler) checkSubscription(subscriptionID string) bool {
	subscription, err := ah.plugin.GetStore().GetChannelSubscription(subscriptionID)
	if err != nil {
		ah.plugin.GetAPI().LogDebug("Unable to get channel subscription", "subscriptionID", subscriptionID, "error", err.Error())
		return false
	}

	if _, err = ah.plugin.GetStore().GetLinkByMSTeamsChannelID(subscription.TeamID, subscription.ChannelID); err != nil {
		ah.plugin.GetAPI().LogDebug("Unable to get the link by MS Teams channel ID", "error", err.Error())
		// Ignoring the error because can be the case that the subscription is no longer exists, in that case, it doesn't matter.
		_ = ah.plugin.GetStore().DeleteSubscription(subscriptionID)
		return false
	}

	return true
}

func (ah *ActivityHandler) handleActivity(activity msteams.Activity) {
	metrics := ah.plugin.GetMetrics()

	activityIds := msteams.GetResourceIds(activity.Resource)

	if activityIds.ChatID == "" {
		if !ah.checkSubscription(activity.SubscriptionID) {
			if metrics != nil {
				metrics.ObserveProcessedChangeEventTotal(activity.ChangeType, discardedReasonExpiredSubscription)
			}
			return
		}
	}

	var discardedReason string
	switch activity.ChangeType {
	case "created":
		discardedReason = ah.handleCreatedActivity(activityIds)
	case "updated":
		discardedReason = ah.handleUpdatedActivity(activityIds)
	case "deleted":
		discardedReason = ah.handleDeletedActivity(activityIds)
	default:
		discardedReason = discardedReasonInvalidChangeType
		ah.plugin.GetAPI().LogError("Unsupported change type", "change_type", activity.ChangeType)
	}

	if metrics != nil {
		metrics.ObserveProcessedChangeEventTotal(activity.ChangeType, discardedReason)
	}
}

func (ah *ActivityHandler) handleCreatedActivity(activityIds msteams.ActivityIds) string {
	msg, chat, err := ah.getMessageAndChatFromActivityIds(activityIds)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to get original message", "error", err.Error())
		return discardedReasonUnableToGetTeamsData
	}

	if msg == nil {
		ah.plugin.GetAPI().LogDebug("Unable to get the message (probably because belongs to private chats of non-connected users)")
		return discardedReasonUnableToGetTeamsData
	}

	if msg.UserID == "" {
		ah.plugin.GetAPI().LogDebug("Skipping not user event", "msg", msg)
		return discardedReasonNotUserEvent
	}

	isDirectMessage := IsDirectMessage(activityIds.ChatID)

	metrics := ah.plugin.GetMetrics()
	// Avoid possible duplication
	postInfo, _ := ah.plugin.GetStore().GetPostInfoByMSTeamsID(msg.ChatID+msg.ChannelID, msg.ID)
	if postInfo != nil {
		ah.plugin.GetAPI().LogDebug("duplicate post")
		ah.updateLastReceivedChangeDate(msg.LastUpdateAt)
		if metrics != nil {
			metrics.ObserveMessagesConfirmedCount(m.ActionSourceMattermost, isDirectMessage)
		}
		return discardedReasonDuplicatedPost
	}

	msteamsUserID, _ := ah.plugin.GetStore().MattermostToTeamsUserID(ah.plugin.GetBotUserID())
	if msg.UserID == msteamsUserID {
		ah.plugin.GetAPI().LogDebug("Skipping messages from bot user")
		ah.updateLastReceivedChangeDate(msg.LastUpdateAt)
		return discardedReasonIsBotUser
	}

	msteamsUser, clientErr := ah.plugin.GetClientForApp().GetUser(msg.UserID)
	if clientErr != nil {
		ah.plugin.GetAPI().LogError("Unable to get the MS Teams user", "error", clientErr.Error())
		return discardedReasonUnableToGetTeamsData
	}

	if msteamsUser.Type == msteamsUserTypeGuest && !ah.plugin.GetSyncGuestUsers() {
		if mmUserID, _ := ah.getOrCreateSyntheticUser(msteamsUser, false); mmUserID != "" && ah.isRemoteUser(mmUserID) {
			if appErr := ah.plugin.GetAPI().UpdateUserActive(mmUserID, false); appErr != nil {
				ah.plugin.GetAPI().LogDebug("Unable to deactivate user", "MMUserID", mmUserID, "Error", appErr.Error())
			}
		}

		return discardedReasonOther
	}

	var senderID string
	var channelID string
	if chat != nil {
		if !ah.plugin.GetSyncDirectMessages() {
			// Skipping because direct/group messages are disabled
			return discardedReasonDirectMessagesDisabled
		}

		channelID, err = ah.getChatChannelID(chat)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to get original channel id", "error", err.Error())
			return discardedReasonOther
		}
		senderID, _ = ah.plugin.GetStore().TeamsToMattermostUserID(msg.UserID)
	} else {
		senderID, _ = ah.getOrCreateSyntheticUser(msteamsUser, true)
		channelLink, _ := ah.plugin.GetStore().GetLinkByMSTeamsChannelID(msg.TeamID, msg.ChannelID)
		if channelLink != nil {
			channelID = channelLink.MattermostChannelID
		}
	}

	if err != nil || senderID == "" {
		senderID = ah.plugin.GetBotUserID()
	}

	if isActiveUser := ah.isActiveUser(senderID); !isActiveUser {
		ah.plugin.GetAPI().LogDebug("Skipping messages from inactive user", "MMUserID", senderID)
		return discardedReasonInactiveUser
	}

	if channelID == "" {
		ah.plugin.GetAPI().LogDebug("Channel not set")
		return discardedReasonOther
	}

	post, errorFound := ah.msgToPost(channelID, senderID, msg, chat, false)
	ah.plugin.GetAPI().LogDebug("Post generated")

	newPost, appErr := ah.plugin.GetAPI().CreatePost(post)
	if appErr != nil {
		ah.plugin.GetAPI().LogError("Unable to create post", "Error", appErr)
		return discardedReasonOther
	}

	if metrics != nil {
		metrics.ObserveMessagesCount(m.ActionCreated, m.ActionSourceMSTeams, isDirectMessage)
	}

	ah.plugin.GetAPI().LogDebug("Post created")
	if errorFound {
		_ = ah.plugin.GetAPI().SendEphemeralPost(senderID, &model.Post{
			ChannelId: channelID,
			UserId:    ah.plugin.GetBotUserID(),
			Message:   "Some images could not be delivered because they exceeded the maximum resolution and/or size allowed.",
		})
	}

	ah.updateLastReceivedChangeDate(msg.LastUpdateAt)
	if newPost != nil && newPost.Id != "" && msg.ID != "" {
		if err := ah.plugin.GetStore().LinkPosts(nil, storemodels.PostInfo{MattermostID: newPost.Id, MSTeamsChannel: fmt.Sprintf(msg.ChatID + msg.ChannelID), MSTeamsID: msg.ID, MSTeamsLastUpdateAt: msg.LastUpdateAt}); err != nil {
			ah.plugin.GetAPI().LogWarn("Error updating the MSTeams/Mattermost post link metadata", "error", err)
		}
	}
	return discardedReasonNone
}

func (ah *ActivityHandler) handleUpdatedActivity(activityIds msteams.ActivityIds) string {
	msg, chat, err := ah.getMessageAndChatFromActivityIds(activityIds)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to get original message", "error", err.Error())
		return discardedReasonUnableToGetTeamsData
	}

	if msg == nil {
		ah.plugin.GetAPI().LogDebug("Unable to get the message (probably because belongs to private chats of non-connected users)")
		return discardedReasonUnableToGetTeamsData
	}

	if msg.UserID == "" {
		ah.plugin.GetAPI().LogDebug("Skipping not user event", "msg", msg)
		return discardedReasonNotUserEvent
	}

	msteamsUserID, _ := ah.plugin.GetStore().MattermostToTeamsUserID(ah.plugin.GetBotUserID())
	if msg.UserID == msteamsUserID {
		ah.plugin.GetAPI().LogDebug("Skipping messages from bot user")
		ah.updateLastReceivedChangeDate(msg.LastUpdateAt)
		return discardedReasonIsBotUser
	}

	postInfo, _ := ah.plugin.GetStore().GetPostInfoByMSTeamsID(msg.ChatID+msg.ChannelID, msg.ID)
	if postInfo == nil {
		ah.updateLastReceivedChangeDate(msg.LastUpdateAt)
		return discardedReasonOther
	}

	// Ignore if the change is already applied in the database
	if postInfo.MSTeamsLastUpdateAt.UnixMicro() == msg.LastUpdateAt.UnixMicro() {
		return discardedReasonAlreadyAppliedChange
	}

	channelID := ""
	if chat == nil {
		var channelLink *storemodels.ChannelLink
		channelLink, err = ah.plugin.GetStore().GetLinkByMSTeamsChannelID(msg.TeamID, msg.ChannelID)
		if err != nil || channelLink == nil {
			ah.plugin.GetAPI().LogError("Unable to find the subscription")
			return discardedReasonOther
		}
		channelID = channelLink.MattermostChannelID
	} else {
		if !ah.plugin.GetSyncDirectMessages() {
			// Skipping because direct/group messages are disabled
			return discardedReasonDirectMessagesDisabled
		}
		post, postErr := ah.plugin.GetAPI().GetPost(postInfo.MattermostID)
		if postErr != nil {
			if strings.EqualFold(postErr.Id, "app.post.get.app_error") {
				if err = ah.plugin.GetStore().RecoverPost(postInfo.MattermostID); err != nil {
					ah.plugin.GetAPI().LogError("Unable to recover the post", "postID", postInfo.MattermostID, "error", err)
					return discardedReasonOther
				}
				post, postErr = ah.plugin.GetAPI().GetPost(postInfo.MattermostID)
				if postErr != nil {
					ah.plugin.GetAPI().LogError("Unable to find the original post after recovery", "postID", postInfo.MattermostID, "error", postErr.Error())
					return discardedReasonOther
				}
			} else {
				ah.plugin.GetAPI().LogError("Unable to find the original post", "error", postErr.Error())
				return discardedReasonOther
			}
		}
		channelID = post.ChannelId
	}

	senderID, err := ah.plugin.GetStore().TeamsToMattermostUserID(msg.UserID)
	if err != nil || senderID == "" {
		senderID = ah.plugin.GetBotUserID()
	}

	if isActiveUser := ah.isActiveUser(senderID); !isActiveUser {
		ah.plugin.GetAPI().LogDebug("Skipping messages from inactive user", "MMUserID", senderID)
		return discardedReasonInactiveUser
	}

	post, _ := ah.msgToPost(channelID, senderID, msg, chat, true)
	post.Id = postInfo.MattermostID

	ah.IgnorePluginHooksMap.Store(fmt.Sprintf("post_%s", post.Id), true)
	if _, appErr := ah.plugin.GetAPI().UpdatePost(post); appErr != nil {
		ah.IgnorePluginHooksMap.Delete(fmt.Sprintf("post_%s", post.Id))
		if strings.EqualFold(appErr.Id, "app.post.get.app_error") {
			if err = ah.plugin.GetStore().RecoverPost(post.Id); err != nil {
				ah.plugin.GetAPI().LogError("Unable to recover the post", "PostID", post.Id, "error", err)
				return discardedReasonOther
			}
		} else {
			ah.plugin.GetAPI().LogError("Unable to update post", "PostID", post.Id, "Error", appErr)
			return discardedReasonOther
		}
	}

	isDirectMessage := IsDirectMessage(activityIds.ChatID)
	metrics := ah.plugin.GetMetrics()
	if metrics != nil {
		metrics.ObserveMessagesCount(m.ActionUpdated, m.ActionSourceMSTeams, isDirectMessage)
	}
	ah.updateLastReceivedChangeDate(msg.LastUpdateAt)
	ah.handleReactions(postInfo.MattermostID, channelID, isDirectMessage, msg.Reactions)
	return discardedReasonNone
}

func (ah *ActivityHandler) handleReactions(postID, channelID string, isDirectMessage bool, reactions []msteams.Reaction) {
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

	metrics := ah.plugin.GetMetrics()
	for _, r := range postReactions {
		if !allReactions[r.UserId+r.EmojiName] {
			r.ChannelId = "removedfromplugin"
			if appErr = ah.plugin.GetAPI().RemoveReaction(r); appErr != nil {
				ah.plugin.GetAPI().LogError("Unable to remove reaction", "error", appErr.Error())
			}
			if metrics != nil {
				metrics.ObserveReactionsCount(m.ReactionUnsetAction, m.ActionSourceMSTeams, isDirectMessage)
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
			ah.IgnorePluginHooksMap.Store(fmt.Sprintf("%s_%s_%s", postID, reactionUserID, emojiName), true)
			r, appErr := ah.plugin.GetAPI().AddReaction(&model.Reaction{
				UserId:    reactionUserID,
				PostId:    postID,
				ChannelId: channelID,
				EmojiName: emojiName,
			})
			if appErr != nil {
				ah.IgnorePluginHooksMap.Delete(fmt.Sprintf("reactions_%s_%s", reactionUserID, emojiName))
				ah.plugin.GetAPI().LogError("failed to create the reaction", "err", appErr)
				continue
			}
			ah.plugin.GetAPI().LogDebug("Added reaction", "reaction", r)
			if metrics != nil {
				metrics.ObserveReactionsCount(m.ReactionSetAction, m.ActionSourceMSTeams, isDirectMessage)
			}
		}
	}
}

func (ah *ActivityHandler) handleDeletedActivity(activityIds msteams.ActivityIds) string {
	messageID := activityIds.MessageID
	if activityIds.ReplyID != "" {
		messageID = activityIds.ReplyID
	}
	postInfo, _ := ah.plugin.GetStore().GetPostInfoByMSTeamsID(activityIds.ChatID+activityIds.ChannelID, messageID)
	if postInfo == nil {
		return discardedReasonOther
	}

	appErr := ah.plugin.GetAPI().DeletePost(postInfo.MattermostID)
	if appErr != nil {
		ah.plugin.GetAPI().LogError("Unable to to delete post", "msgID", postInfo.MattermostID, "error", appErr)
		return discardedReasonOther
	}

	metrics := ah.plugin.GetMetrics()
	if metrics != nil {
		metrics.ObserveMessagesCount(m.ActionDeleted, m.ActionSourceMSTeams, IsDirectMessage(activityIds.ChatID))
	}
	return discardedReasonNone
}

func (ah *ActivityHandler) updateLastReceivedChangeDate(t time.Time) {
	err := ah.plugin.GetAPI().KVSet(lastReceivedChangeKey, []byte(strconv.FormatInt(t.UnixMicro(), 10)))
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to store properly the last received change")
	}
}

func (ah *ActivityHandler) isActiveUser(userID string) bool {
	mmUser, err := ah.plugin.GetAPI().GetUser(userID)
	if err != nil {
		ah.plugin.GetAPI().LogWarn("Unable to get Mattermost user", "mmuserID", userID, "error", err.Error())
		return false
	}

	if mmUser.DeleteAt != 0 {
		return false
	}

	return true
}

func (ah *ActivityHandler) isRemoteUser(userID string) bool {
	user, userErr := ah.plugin.GetAPI().GetUser(userID)
	if userErr != nil {
		ah.plugin.GetAPI().LogDebug("Unable to get MM user", "mmuserID", userID, "error", userErr.Error())
		return false
	}

	return user.RemoteId != nil && *user.RemoteId != "" && strings.HasPrefix(user.Username, "msteams_")
}

func IsDirectMessage(chatID string) bool {
	return chatID != ""
}
