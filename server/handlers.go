package main

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/enescakir/emoji"
	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
)

var emojisReverseMap map[string]string

var attachRE = regexp.MustCompile(`<attachment id=.*?attachment>`)
var imageRE = regexp.MustCompile(`<img .*?>`)

const (
	numberOfWorkers             = 50
	activityQueueSize           = 5000
	maxFileAttachmentsSupported = 10
)

type ActivityHandler struct {
	plugin               *Plugin
	queue                chan msteams.Activity
	quit                 chan bool
	workersWaitGroup     sync.WaitGroup
	IgnorePluginHooksMap sync.Map
	lastUpdateAtMap      sync.Map
}

func NewActivityHandler(plugin *Plugin) *ActivityHandler {
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
					ah.plugin.GetAPI().LogWarn("Error storing the subscription last activity at", "error", err, "subscription_id", subscriptionID.(string), "last_update_at", lastUpdateAt.(time.Time))
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
	if event.LifecycleEvent != "reauthorizationRequired" {
		ah.plugin.GetAPI().LogWarn("Ignoring unknown lifecycle event", "lifecycle_event", event.LifecycleEvent)
		ah.plugin.GetMetrics().ObserveLifecycleEvent(event.LifecycleEvent, metrics.DiscardedReasonUnknownLifecycleEvent)
		return
	}

	// Ignore subscriptions we aren't tracking locally. For now, that's just the single global chats subscription.
	if _, err := ah.plugin.GetStore().GetGlobalSubscription(event.SubscriptionID); err == sql.ErrNoRows {
		ah.plugin.GetAPI().LogWarn("Ignoring reauthorizationRequired lifecycle event for unused subscription", "subscription_id", event.SubscriptionID)
		ah.plugin.GetMetrics().ObserveLifecycleEvent(event.LifecycleEvent, metrics.DiscardedReasonUnusedSubscription)
		return
	} else if err != nil {
		ah.plugin.GetAPI().LogWarn("Failed to lookup subscription, refreshing anyway", "subscription_id", event.SubscriptionID, "error", err.Error())
	}

	ah.plugin.GetAPI().LogInfo("Refreshing subscription", "subscription_id", event.SubscriptionID)
	expiresOn, err := ah.plugin.GetClientForApp().RefreshSubscription(event.SubscriptionID)
	if err != nil {
		ah.plugin.GetAPI().LogWarn("Unable to refresh the subscription", "subscription_id", event.SubscriptionID, "error", err.Error())
		ah.plugin.GetMetrics().ObserveLifecycleEvent(event.LifecycleEvent, metrics.DiscardedReasonFailedToRefresh)
		return
	}

	ah.plugin.GetAPI().LogInfo("Refreshed subscription", "subscription_id", event.SubscriptionID, "expires_on", expiresOn.Format("2006-01-02 15:04:05.000 Z07:00"))
	ah.plugin.GetMetrics().ObserveSubscription(metrics.SubscriptionRefreshed)

	if err = ah.plugin.GetStore().UpdateSubscriptionExpiresOn(event.SubscriptionID, *expiresOn); err != nil {
		ah.plugin.GetAPI().LogWarn("Unable to store the subscription new expiry date", "subscription_id", event.SubscriptionID, "error", err.Error())
	}

	ah.plugin.GetMetrics().ObserveLifecycleEvent(event.LifecycleEvent, metrics.DiscardedReasonNone)
}

func (ah *ActivityHandler) handleActivity(activity msteams.Activity) {
	done := ah.plugin.GetMetrics().ObserveWorker(metrics.WorkerActivityHandler)
	defer done()

	activityIds := msteams.GetResourceIds(activity.Resource)

	var discardedReason string
	switch activity.ChangeType {
	case "created":
		discardedReason = ah.handleCreatedActivity(activityIds)
	case "updated":
		discardedReason = metrics.DiscardedReasonNotificationsOnly
	case "deleted":
		discardedReason = metrics.DiscardedReasonNotificationsOnly
	default:
		discardedReason = metrics.DiscardedReasonInvalidChangeType
		ah.plugin.GetAPI().LogWarn("Unsupported change type", "change_type", activity.ChangeType)
	}

	ah.plugin.GetMetrics().ObserveChangeEvent(activity.ChangeType, discardedReason)
}

// handleCreatedActivity handles subscription change events of the created type, i.e. new messages.
func (ah *ActivityHandler) handleCreatedActivity(activityIds clientmodels.ActivityIds) string {
	// We're only handling chats at that time.
	if activityIds.ChatID == "" {
		return metrics.DiscardedReasonChannelNotificationsUnsupported
	}

	// Use the application client to resolve the chat metadata.
	chat, err := ah.plugin.GetClientForApp().GetChat(activityIds.ChatID)
	if err != nil || chat == nil {
		ah.plugin.GetAPI().LogWarn("Failed to get chat", "chat_id", activityIds.ChatID, "error", err)
		return metrics.DiscardedReasonUnableToGetTeamsData
	}

	// Find a connected member whose client can be used to fetch the chat message itself.
	var client msteams.Client
	for _, member := range chat.Members {
		client, _ = ah.plugin.GetClientForTeamsUser(member.UserID)
		if client != nil {
			break
		}
	}
	if client == nil {
		return metrics.DiscardedReasonNoConnectedUser
	}

	// Fetch the message itself.
	msg, err := client.GetChatMessage(chat.ID, activityIds.MessageID)
	if err != nil {
		ah.plugin.GetAPI().LogWarn("Failed to get message from chat", "chat_id", chat.ID, "message_id", activityIds.MessageID, "error", err)
		return metrics.DiscardedReasonUnableToGetTeamsData
	}

	// Skip messages without a user, if this ever happens.
	if msg.UserID == "" {
		return metrics.DiscardedReasonNotUserEvent
	}

	// Finally, process the notification of the chat message received.
	return ah.handleCreatedActivityNotification(msg, chat)
}
