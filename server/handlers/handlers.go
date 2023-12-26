//go:generate mockery --name=PluginIface

package handlers

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/enescakir/emoji"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

var emojisReverseMap map[string]string

var attachRE = regexp.MustCompile(`<attachment id=.*?attachment>`)
var imageRE = regexp.MustCompile(`<img .*?>`)

const (
	numberOfWorkers             = 50
	activityQueueSize           = 5000
	msteamsUserTypeGuest        = "Guest"
	maxFileAttachmentsSupported = 10
)

type PluginIface interface {
	GetAPI() plugin.API
	GetStore() store.Store
	GetMetrics() metrics.Metrics
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
	workersWaitGroup     sync.WaitGroup
	IgnorePluginHooksMap sync.Map
	lastUpdateAtMap      sync.Map
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
	ah.quit = make(chan bool)

	// This is constant for now, but report it as a metric to future proof dashboards.
	ah.plugin.GetMetrics().ObserveChangeEventQueueCapacity(activityQueueSize)

	// doStart is the meat of the activity handler worker
	doStart := func() {
		for {
			select {
			case activity := <-ah.queue:
				ah.plugin.GetMetrics().DecrementChangeEventQueueLength(activity.ChangeType)
				ah.handleActivity(activity)
			case <-ah.quit:
				// we have received a signal to stop
				return
			}
		}
	}

	// doQuit is called when the worker quits intentionally
	doQuit := func() {
		ah.workersWaitGroup.Done()
	}

	// doStart is the meat of the activity handler worker
	doStartLastActivityAt := func() {
		updateLastActivityAt := func(subscriptionID, lastUpdateAt any) bool {
			if time.Since(lastUpdateAt.(time.Time)) <= 5*time.Minute {
				if err := ah.plugin.GetStore().UpdateSubscriptionLastActivityAt(subscriptionID.(string), lastUpdateAt.(time.Time)); err != nil {
					ah.plugin.GetAPI().LogWarn("Error storing the subscription last activity at", "error", err, "subscriptionID", subscriptionID.(string), "lastUpdateAt", lastUpdateAt.(time.Time))
				}
			}
			return true
		}
		for {
			timer := time.NewTimer(5 * time.Minute)
			select {
			case <-timer.C:
				ah.lastUpdateAtMap.Range(updateLastActivityAt)
			case <-ah.quit:
				// we have received a signal to stop
				timer.Stop()
				ah.lastUpdateAtMap.Range(updateLastActivityAt)
				return
			}
		}
	}

	// isQuitting informs the recovery handler if the shutdown is intentional
	isQuitting := func() bool {
		select {
		case <-ah.quit:
			return true
		default:
			return false
		}
	}

	logError := ah.plugin.GetAPI().LogError

	for i := 0; i < numberOfWorkers; i++ {
		ah.workersWaitGroup.Add(1)
		startWorker(logError, ah.plugin.GetMetrics(), isQuitting, doStart, doQuit)
	}
	ah.workersWaitGroup.Add(1)
	startWorker(logError, ah.plugin.GetMetrics(), isQuitting, doStartLastActivityAt, doQuit)
}

func (ah *ActivityHandler) Stop() {
	close(ah.quit)
	ah.workersWaitGroup.Wait()
}

func (ah *ActivityHandler) Handle(activity msteams.Activity) error {
	select {
	case ah.queue <- activity:
		ah.plugin.GetMetrics().IncrementChangeEventQueueLength(activity.ChangeType)
	default:
		ah.plugin.GetMetrics().ObserveChangeEventQueueRejected()
		return errors.New("activity queue size full")
	}

	return nil
}

func (ah *ActivityHandler) HandleLifecycleEvent(event msteams.Activity) {
	if !ah.checkSubscription(event.SubscriptionID) {
		ah.plugin.GetMetrics().ObserveLifecycleEvent(event.LifecycleEvent, metrics.DiscardedReasonFailedSubscriptionCheck)
		return
	}

	if event.LifecycleEvent == "reauthorizationRequired" {
		expiresOn, err := ah.plugin.GetClientForApp().RefreshSubscription(event.SubscriptionID)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to refresh the subscription", "error", err.Error())
			ah.plugin.GetMetrics().ObserveLifecycleEvent(event.LifecycleEvent, metrics.DiscardedReasonFailedToRefresh)
			return
		}

		ah.plugin.GetMetrics().ObserveSubscription(metrics.SubscriptionRefreshed)
		if err = ah.plugin.GetStore().UpdateSubscriptionExpiresOn(event.SubscriptionID, *expiresOn); err != nil {
			ah.plugin.GetAPI().LogError("Unable to store the subscription new expiry date", "subscriptionID", event.SubscriptionID, "error", err.Error())
		}
	}

	ah.plugin.GetMetrics().ObserveLifecycleEvent(event.LifecycleEvent, metrics.DiscardedReasonNone)
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
	activityIds := msteams.GetResourceIds(activity.Resource)

	if activityIds.ChatID == "" {
		if !ah.checkSubscription(activity.SubscriptionID) {
			ah.plugin.GetMetrics().ObserveChangeEvent(activity.ChangeType, metrics.DiscardedReasonExpiredSubscription)
			return
		}
	}

	var discardedReason string
	switch activity.ChangeType {
	case "created":
		var msg *clientmodels.Message
		if len(activity.Content) > 0 {
			var err error
			msg, err = msteams.GetMessageFromJSON(activity.Content, activityIds.TeamID, activityIds.ChannelID, activityIds.ChatID)
			if err != nil {
				ah.plugin.GetAPI().LogDebug("Unable to unmarshal activity message", "activity", activity, "error", err)
			}
		}
		discardedReason = ah.handleCreatedActivity(msg, activity.SubscriptionID, activityIds)
	case "updated":
		var msg *clientmodels.Message
		if len(activity.Content) > 0 {
			var err error
			msg, err = msteams.GetMessageFromJSON(activity.Content, activityIds.TeamID, activityIds.ChannelID, activityIds.ChatID)
			if err != nil {
				ah.plugin.GetAPI().LogDebug("Unable to unmarshal activity message", "activity", activity, "error", err)
			}
		}
		discardedReason = ah.handleUpdatedActivity(msg, activity.SubscriptionID, activityIds)
	case "deleted":
		discardedReason = ah.handleDeletedActivity(activityIds)
	default:
		discardedReason = metrics.DiscardedReasonInvalidChangeType
		ah.plugin.GetAPI().LogError("Unsupported change type", "change_type", activity.ChangeType)
	}

	ah.plugin.GetMetrics().ObserveChangeEvent(activity.ChangeType, discardedReason)
}

func (ah *ActivityHandler) handleCreatedActivity(msg *clientmodels.Message, subscriptionID string, activityIds clientmodels.ActivityIds) string {
	msg, chat, err := ah.getMessageAndChatFromActivityIds(msg, activityIds)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to get original message", "error", err.Error())
		return metrics.DiscardedReasonUnableToGetTeamsData
	}

	if msg == nil {
		ah.plugin.GetAPI().LogDebug("Unable to get the message (probably because belongs to private chats of non-connected users)")
		return metrics.DiscardedReasonUnableToGetTeamsData
	}

	if msg.UserID == "" {
		ah.plugin.GetAPI().LogDebug("Skipping not user event", "msg", msg)
		return metrics.DiscardedReasonNotUserEvent
	}

	isDirectMessage := IsDirectMessage(activityIds.ChatID)

	// Avoid possible duplication
	postInfo, _ := ah.plugin.GetStore().GetPostInfoByMSTeamsID(msg.ChatID+msg.ChannelID, msg.ID)
	if postInfo != nil {
		ah.plugin.GetAPI().LogDebug("duplicate post")
		ah.plugin.GetMetrics().ObserveConfirmedMessage(metrics.ActionSourceMattermost, isDirectMessage)
		return metrics.DiscardedReasonDuplicatedPost
	}

	msteamsUserID, _ := ah.plugin.GetStore().MattermostToTeamsUserID(ah.plugin.GetBotUserID())
	if msg.UserID == msteamsUserID {
		ah.plugin.GetAPI().LogDebug("Skipping messages from bot user")
		return metrics.DiscardedReasonIsBotUser
	}

	msteamsUser, clientErr := ah.plugin.GetClientForApp().GetUser(msg.UserID)
	if clientErr != nil {
		ah.plugin.GetAPI().LogError("Unable to get the MS Teams user", "error", clientErr.Error())
		return metrics.DiscardedReasonUnableToGetTeamsData
	}

	if msteamsUser.Type == msteamsUserTypeGuest && !ah.plugin.GetSyncGuestUsers() {
		if mmUserID, _ := ah.getOrCreateSyntheticUser(msteamsUser, false); mmUserID != "" && ah.isRemoteUser(mmUserID) {
			if appErr := ah.plugin.GetAPI().UpdateUserActive(mmUserID, false); appErr != nil {
				ah.plugin.GetAPI().LogDebug("Unable to deactivate user", "MMUserID", mmUserID, "Error", appErr.Error())
			}
		}

		return metrics.DiscardedReasonOther
	}

	var senderID string
	var channelID string
	if chat != nil {
		if !ah.plugin.GetSyncDirectMessages() {
			// Skipping because direct/group messages are disabled
			return metrics.DiscardedReasonDirectMessagesDisabled
		}

		channelID, err = ah.getChatChannelID(chat)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to get original channel id", "error", err.Error())
			return metrics.DiscardedReasonOther
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
		return metrics.DiscardedReasonInactiveUser
	}

	if channelID == "" {
		ah.plugin.GetAPI().LogDebug("Channel not set")
		return metrics.DiscardedReasonOther
	}

	post, errorFound := ah.msgToPost(channelID, senderID, msg, chat, false)
	ah.plugin.GetAPI().LogDebug("Post generated")

	newPost, appErr := ah.plugin.GetAPI().CreatePost(post)
	if appErr != nil {
		ah.plugin.GetAPI().LogError("Unable to create post", "Error", appErr)
		return metrics.DiscardedReasonOther
	}

	ah.plugin.GetMetrics().ObserveMessage(metrics.ActionCreated, metrics.ActionSourceMSTeams, isDirectMessage)

	ah.plugin.GetAPI().LogDebug("Post created")
	if errorFound {
		_ = ah.plugin.GetAPI().SendEphemeralPost(senderID, &model.Post{
			ChannelId: channelID,
			UserId:    ah.plugin.GetBotUserID(),
			Message:   "Some images could not be delivered because they exceeded the maximum resolution and/or size allowed.",
		})
	}

	if newPost != nil && newPost.Id != "" && msg.ID != "" {
		if err := ah.plugin.GetStore().LinkPosts(nil, storemodels.PostInfo{MattermostID: newPost.Id, MSTeamsChannel: fmt.Sprintf(msg.ChatID + msg.ChannelID), MSTeamsID: msg.ID, MSTeamsLastUpdateAt: msg.LastUpdateAt}); err != nil {
			ah.plugin.GetAPI().LogWarn("Error updating the MSTeams/Mattermost post link metadata", "error", err)
		}
	}

	ah.lastUpdateAtMap.Store(subscriptionID, msg.LastUpdateAt)
	return metrics.DiscardedReasonNone
}

func (ah *ActivityHandler) handleUpdatedActivity(msg *clientmodels.Message, subscriptionID string, activityIds clientmodels.ActivityIds) string {
	msg, chat, err := ah.getMessageAndChatFromActivityIds(msg, activityIds)
	if err != nil {
		ah.plugin.GetAPI().LogError("Unable to get original message", "error", err.Error())
		return metrics.DiscardedReasonUnableToGetTeamsData
	}

	if msg == nil {
		ah.plugin.GetAPI().LogDebug("Unable to get the message (probably because belongs to private chats of non-connected users)")
		return metrics.DiscardedReasonUnableToGetTeamsData
	}

	if msg.UserID == "" {
		ah.plugin.GetAPI().LogDebug("Skipping not user event", "msg", msg)
		return metrics.DiscardedReasonNotUserEvent
	}

	msteamsUserID, _ := ah.plugin.GetStore().MattermostToTeamsUserID(ah.plugin.GetBotUserID())
	if msg.UserID == msteamsUserID {
		ah.plugin.GetAPI().LogDebug("Skipping messages from bot user")
		return metrics.DiscardedReasonIsBotUser
	}

	postInfo, _ := ah.plugin.GetStore().GetPostInfoByMSTeamsID(msg.ChatID+msg.ChannelID, msg.ID)
	if postInfo == nil {
		return metrics.DiscardedReasonOther
	}

	// Ignore if the change is already applied in the database
	if postInfo.MSTeamsLastUpdateAt.UnixMicro() == msg.LastUpdateAt.UnixMicro() {
		return metrics.DiscardedReasonAlreadyAppliedChange
	}

	channelID := ""
	if chat == nil {
		var channelLink *storemodels.ChannelLink
		channelLink, err = ah.plugin.GetStore().GetLinkByMSTeamsChannelID(msg.TeamID, msg.ChannelID)
		if err != nil || channelLink == nil {
			ah.plugin.GetAPI().LogError("Unable to find the subscription")
			return metrics.DiscardedReasonOther
		}
		channelID = channelLink.MattermostChannelID
	} else {
		if !ah.plugin.GetSyncDirectMessages() {
			// Skipping because direct/group messages are disabled
			return metrics.DiscardedReasonDirectMessagesDisabled
		}
		post, postErr := ah.plugin.GetAPI().GetPost(postInfo.MattermostID)
		if postErr != nil {
			if strings.EqualFold(postErr.Id, "app.post.get.app_error") {
				if err = ah.plugin.GetStore().RecoverPost(postInfo.MattermostID); err != nil {
					ah.plugin.GetAPI().LogError("Unable to recover the post", "postID", postInfo.MattermostID, "error", err)
					return metrics.DiscardedReasonOther
				}
				post, postErr = ah.plugin.GetAPI().GetPost(postInfo.MattermostID)
				if postErr != nil {
					ah.plugin.GetAPI().LogError("Unable to find the original post after recovery", "postID", postInfo.MattermostID, "error", postErr.Error())
					return metrics.DiscardedReasonOther
				}
			} else {
				ah.plugin.GetAPI().LogError("Unable to find the original post", "error", postErr.Error())
				return metrics.DiscardedReasonOther
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
		return metrics.DiscardedReasonInactiveUser
	}

	post, _ := ah.msgToPost(channelID, senderID, msg, chat, true)
	post.Id = postInfo.MattermostID

	ah.IgnorePluginHooksMap.Store(fmt.Sprintf("post_%s", post.Id), true)
	if _, appErr := ah.plugin.GetAPI().UpdatePost(post); appErr != nil {
		ah.IgnorePluginHooksMap.Delete(fmt.Sprintf("post_%s", post.Id))
		if strings.EqualFold(appErr.Id, "app.post.get.app_error") {
			if err = ah.plugin.GetStore().RecoverPost(post.Id); err != nil {
				ah.plugin.GetAPI().LogError("Unable to recover the post", "PostID", post.Id, "error", err)
				return metrics.DiscardedReasonOther
			}
		} else {
			ah.plugin.GetAPI().LogError("Unable to update post", "PostID", post.Id, "Error", appErr)
			return metrics.DiscardedReasonOther
		}
	}

	isDirectMessage := IsDirectMessage(activityIds.ChatID)
	ah.plugin.GetMetrics().ObserveMessage(metrics.ActionUpdated, metrics.ActionSourceMSTeams, isDirectMessage)
	ah.handleReactions(postInfo.MattermostID, channelID, isDirectMessage, msg.Reactions)

	ah.lastUpdateAtMap.Store(subscriptionID, msg.LastUpdateAt)
	return metrics.DiscardedReasonNone
}

func (ah *ActivityHandler) handleReactions(postID, channelID string, isDirectMessage bool, reactions []clientmodels.Reaction) {
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
			if appErr = ah.plugin.GetAPI().RemoveReaction(r); appErr != nil {
				ah.plugin.GetAPI().LogError("Unable to remove reaction", "error", appErr.Error())
			}
			ah.plugin.GetMetrics().ObserveReaction(metrics.ReactionUnsetAction, metrics.ActionSourceMSTeams, isDirectMessage)
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
			ah.plugin.GetMetrics().ObserveReaction(metrics.ReactionSetAction, metrics.ActionSourceMSTeams, isDirectMessage)
		}
	}
}

func (ah *ActivityHandler) handleDeletedActivity(activityIds clientmodels.ActivityIds) string {
	messageID := activityIds.MessageID
	if activityIds.ReplyID != "" {
		messageID = activityIds.ReplyID
	}
	postInfo, _ := ah.plugin.GetStore().GetPostInfoByMSTeamsID(activityIds.ChatID+activityIds.ChannelID, messageID)
	if postInfo == nil {
		return metrics.DiscardedReasonOther
	}

	appErr := ah.plugin.GetAPI().DeletePost(postInfo.MattermostID)
	if appErr != nil {
		ah.plugin.GetAPI().LogError("Unable to to delete post", "msgID", postInfo.MattermostID, "error", appErr)
		return metrics.DiscardedReasonOther
	}

	ah.plugin.GetMetrics().ObserveMessage(metrics.ActionDeleted, metrics.ActionSourceMSTeams, IsDirectMessage(activityIds.ChatID))

	return metrics.DiscardedReasonNone
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
