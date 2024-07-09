package main

import (
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
	if !ah.checkSubscription(event.SubscriptionID) {
		ah.plugin.GetMetrics().ObserveLifecycleEvent(event.LifecycleEvent, metrics.DiscardedReasonFailedSubscriptionCheck)
		return
	}

	if event.LifecycleEvent == "reauthorizationRequired" {
		expiresOn, err := ah.plugin.GetClientForApp().RefreshSubscription(event.SubscriptionID)
		if err != nil {
			ah.plugin.GetAPI().LogWarn("Unable to refresh the subscription", "error", err.Error())
			ah.plugin.GetMetrics().ObserveLifecycleEvent(event.LifecycleEvent, metrics.DiscardedReasonFailedToRefresh)
			return
		}

		ah.plugin.GetMetrics().ObserveSubscription(metrics.SubscriptionRefreshed)
		if err = ah.plugin.GetStore().UpdateSubscriptionExpiresOn(event.SubscriptionID, *expiresOn); err != nil {
			ah.plugin.GetAPI().LogWarn("Unable to store the subscription new expiry date", "subscription_id", event.SubscriptionID, "error", err.Error())
		}
	}

	ah.plugin.GetMetrics().ObserveLifecycleEvent(event.LifecycleEvent, metrics.DiscardedReasonNone)
}

func (ah *ActivityHandler) checkSubscription(subscriptionID string) bool {
	subscription, err := ah.plugin.GetStore().GetChannelSubscription(subscriptionID)
	if err != nil {
		ah.plugin.GetAPI().LogWarn("Unable to get channel subscription", "subscription_id", subscriptionID, "error", err.Error())
		return false
	}

	if _, err = ah.plugin.GetStore().GetLinkByMSTeamsChannelID(subscription.TeamID, subscription.ChannelID); err != nil {
		ah.plugin.GetAPI().LogWarn("Unable to get the link by MS Teams channel ID", "error", err.Error())
		// Ignoring the error because can be the case that the subscription is no longer exists, in that case, it doesn't matter.
		_ = ah.plugin.GetStore().DeleteSubscription(subscriptionID)
		return false
	}

	return true
}

func (ah *ActivityHandler) handleActivity(activity msteams.Activity) {
	done := ah.plugin.GetMetrics().ObserveWorker(metrics.WorkerActivityHandler)
	defer done()

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
				ah.plugin.GetAPI().LogWarn("Unable to unmarshal activity message", "activity", activity, "error", err)
			}
		}
		discardedReason = ah.handleCreatedActivity(msg, activity.SubscriptionID, activityIds)
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

func (ah *ActivityHandler) handleCreatedActivity(msg *clientmodels.Message, subscriptionID string, activityIds clientmodels.ActivityIds) string {
	msg, chat, err := ah.getMessageAndChatFromActivityIds(msg, activityIds)
	if err != nil {
		ah.plugin.GetAPI().LogWarn("Unable to get original message", "error", err.Error())
		return metrics.DiscardedReasonUnableToGetTeamsData
	}
	if msg == nil {
		return metrics.DiscardedReasonUnableToGetTeamsData
	}

	if msg.UserID == "" {
		return metrics.DiscardedReasonNotUserEvent
	}

	return ah.handleCreatedActivityNotification(msg, chat)
}
