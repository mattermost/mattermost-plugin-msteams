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
	"github.com/pkg/errors"
)

var emojisReverseMap map[string]string

var attachRE = regexp.MustCompile(`<attachment id=.*?attachment>`)

const (
	lastReceivedChangeKey = "last_received_change"
	numberOfWorkers       = 20
	activityQueueSize     = 1000
)

type pluginIface interface {
	GetAPI() plugin.API
	GetStore() store.Store
	GetWebhookSecret() string
	GetSyncDirectMessages() bool
	GetBotUserID() string
	GetURL() string
	GetClientForApp() msteams.Client
	GetClientForUser(string) (msteams.Client, error)
	GetClientForTeamsUser(string) (msteams.Client, error)
}

type ActivityHandler struct {
	plugin pluginIface
	queue  chan msteams.Activity
	quit   chan bool
}

func New(plugin pluginIface) *ActivityHandler {
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

	return &ActivityHandler{
		plugin: plugin,
		queue:  make(chan msteams.Activity, activityQueueSize),
		quit:   make(chan bool),
	}
}

func (ah *ActivityHandler) Start() {
	for i := 0; i < numberOfWorkers; i++ {
		go func() {
			select {
			case activity := <-ah.queue:
				ah.handleActivity(activity)
			case <-ah.quit:
				// we have received a signal to stop
				return
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
	if activity.ClientState != ah.plugin.GetWebhookSecret() {
		ah.plugin.GetAPI().LogError("Unable to process activity", "activity", activity, "error", "Invalid webhook secret")
		return errors.New("Invalid webhook secret")
	}
	ah.queue <- activity
	return nil
}

func (ah *ActivityHandler) handleActivity(activity msteams.Activity) {
	activityIds := msteams.GetResourceIds(activity.Resource)

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
		ah.plugin.GetAPI().LogDebug("Unable to get the message (probably because belongs to private chate in not-linked users)")
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

	var channelID string
	senderID, err := ah.plugin.GetStore().TeamsToMattermostUserID(msg.UserID)
	if err != nil || senderID == "" {
		senderID = ah.plugin.GetBotUserID()
	}
	if chat != nil {
		if !ah.plugin.GetSyncDirectMessages() {
			// Skipping because direct/group messages are disabled
			return
		}
		channelID, err = ah.getChatChannelID(chat, msg.UserID)
		if err != nil {
			ah.plugin.GetAPI().LogError("Unable to get original channel id", "error", err.Error())
			return
		}
	} else {
		channelLink, _ := ah.plugin.GetStore().GetLinkByMSTeamsChannelID(msg.TeamID, msg.ChannelID)
		if channelLink != nil {
			channelID = channelLink.MattermostChannel
		}
	}

	if channelID == "" {
		ah.plugin.GetAPI().LogDebug("Channel not set")
		return
	}

	post, err := ah.msgToPost(channelID, msg, senderID)
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
		ah.plugin.GetAPI().LogDebug("Unable to get the message (probably because belongs to private chate in not-linked users)")
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
		channelID = channelLink.MattermostChannel
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

	post, err := ah.msgToPost(channelID, msg, senderID)
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
	if len(reactions) == 0 {
		return
	}
	ah.plugin.GetAPI().LogDebug("Handling reactions", "reactions", reactions)

	postReactions, appErr := ah.plugin.GetAPI().GetReactions(postID)
	if appErr != nil {
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
			ah.plugin.GetAPI().LogError("Not code reaction found for reaction", "reaction", reaction.Reaction)
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
			ah.plugin.GetAPI().LogError("Not code reaction found for reaction", "reaction", reaction.Reaction)
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
	postInfo, _ := ah.plugin.GetStore().GetPostInfoByMSTeamsID(activityIds.ChatID+activityIds.ChannelID, activityIds.MessageID)
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
