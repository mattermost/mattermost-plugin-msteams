//go:generate mockery --name=Metrics
package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

const (
	MetricsNamespace        = "msteams_connect"
	MetricsSubsystemSystem  = "system"
	MetricsSubsystemApp     = "app"
	MetricsSubsystemHTTP    = "http"
	MetricsSubsystemAPI     = "api"
	MetricsSubsystemEvents  = "events"
	MetricsSubsystemDB      = "db"
	MetricsSubsystemMSGraph = "msgraph"

	MetricsCloudInstallationLabel = "installationId"
	MetricsVersionLabel           = "version"

	ActionSourceMSTeams    = "msteams"
	ActionSourceMattermost = "mattermost"
	ActionCreated          = "created"
	ActionUpdated          = "updated"
	ActionDeleted          = "deleted"
	ReactionSetAction      = "set"
	ReactionUnsetAction    = "unset"
	SubscriptionRefreshed  = "refreshed"
	SubscriptionConnected  = "connected"
	SubscriptionDeleted    = "deleted"

	DiscardedReasonNone                            = ""
	DiscardedReasonInvalidChangeType               = "invalid_change_type"
	DiscardedReasonUnableToGetTeamsData            = "unable_to_get_teams_data" // #nosec  false positive
	DiscardedReasonNotUserEvent                    = "no_user_event"
	DiscardedReasonFileLimitReached                = "file_limit_reached"
	DiscardedReasonEmptyFileID                     = "empty_file_id"
	DiscardedReasonMaxFileSizeExceeded             = "max_file_size_exceeded"
	DiscardedReasonExpiredSubscription             = "expired_subscription"
	DiscardedReasonInvalidWebhookSecret            = "invalid_webhook_secret"
	DiscardedReasonFailedSubscriptionCheck         = "failed_subscription_check"
	DiscardedReasonFailedToRefresh                 = "failed_to_refresh"
	DiscardedReasonNotificationsOnly               = "notifications_only"
	DiscardedReasonChannelNotificationsUnsupported = "channel_notifications_unsupported"
	DiscardedReasonNoConnectedUser                 = "no_connected_user"

	WorkerMonitor          = "monitor"
	WorkerActivityHandler  = "activity_handler"
	WorkerCheckCredentials = "check_credentials" //#nosec G101 -- This is a false positive
)

type Metrics interface {
	ObserveAPIEndpointDuration(handler, method, statusCode string, elapsed float64)

	IncrementHTTPRequests()
	IncrementHTTPErrors()
	ObserveOAuthTokenInvalidated()
	ObserveChangeEventQueueRejected()

	ObserveChangeEvent(changeType string, discardedReason string)
	ObserveLifecycleEvent(lifecycleEventType, discardedReason string)
	ObserveMessage(action, source string, isDirectOrGroupMessage bool)
	ObserveMessageDelay(action, source string, isDirectOrGroupMessage bool, delay time.Duration)
	ObserveReaction(action, source string, isDirectOrGroupMessage bool)
	ObserveFiles(action, source, discardedReason string, isDirectOrGroupMessage bool, count int64)
	ObserveFile(action, source, discardedReason string, isDirectOrGroupMessage bool)
	ObserveSubscription(action string)

	ObserveConnectedUsers(count int64)
	ObserveConnectedUsersLimit(count int64)
	ObservePendingInvites(count int64)
	ObservePendingInvitesLimit(count int64)
	ObserveWhitelistedUsers(count int64)

	ObserveLinkedChannels(count int64)
	ObserveActiveUsersReceiving(count int64)

	ObserveChangeEventQueueCapacity(count int64)
	IncrementChangeEventQueueLength(changeType string)
	DecrementChangeEventQueueLength(changeType string)

	ObserveMSGraphClientMethodDuration(method, success, statusCode string, elapsed float64)
	ObserveStoreMethodDuration(method, success string, elapsed float64)

	GetRegistry() *prometheus.Registry

	ObserveGoroutineFailure()
	IncrementActiveWorkers(worker string)
	DecrementActiveWorkers(worker string)
	ObserveWorkerDuration(worker string, elapsed float64)
	ObserveWorker(worker string) func()
	ObserveClientSecretEndDateTime(expireDate time.Time)
	ObserveSyncMsgPostDelay(action string, delayMillis int64)
	ObserveSyncMsgReactionDelay(action string, delayMillis int64)
	ObserveSyncMsgFileDelay(action string, delayMillis int64)
	ObserveNotification(isGroupChat, hasAttachments bool)
}

type InstanceInfo struct {
	InstallationID      string
	ConnectedUsersLimit int
	PluginVersion       string
}

// metrics used to instrumentate metrics in prometheus.
type metrics struct {
	registry *prometheus.Registry

	pluginStartTime        prometheus.Gauge
	pluginInfo             prometheus.Gauge
	goroutineFailuresTotal prometheus.Counter

	apiTime *prometheus.HistogramVec

	msGraphClientTime *prometheus.HistogramVec

	httpRequestsTotal          prometheus.Counter
	httpErrorsTotal            prometheus.Counter
	oAuthTokenInvalidatedTotal prometheus.Counter

	lifecycleEventsTotal     *prometheus.CounterVec
	changeEventsTotal        *prometheus.CounterVec
	messagesTotal            *prometheus.CounterVec
	messageDelayTime         *prometheus.HistogramVec
	reactionsTotal           *prometheus.CounterVec
	filesTotal               *prometheus.CounterVec
	subscriptionsTotal       *prometheus.CounterVec
	syncMsgPostDelayTime     *prometheus.HistogramVec
	syncMsgReactionDelayTime *prometheus.HistogramVec
	syncMsgFileDelayTime     *prometheus.HistogramVec

	connectedUsers      prometheus.Gauge
	connectedUsersLimit prometheus.Gauge
	pendingInvites      prometheus.Gauge
	pendingInvitesLimit prometheus.Gauge
	whitelistedUsers    prometheus.Gauge

	linkedChannels prometheus.Gauge

	activeUsersReceiving prometheus.Gauge

	changeEventQueueCapacity      prometheus.Gauge
	changeEventQueueLength        *prometheus.GaugeVec
	changeEventQueueRejectedTotal prometheus.Counter
	activeWorkersTotal            *prometheus.GaugeVec
	clientSecretEndDateTime       prometheus.Gauge

	storeTime          *prometheus.HistogramVec
	workersTime        *prometheus.HistogramVec
	notificationsTotal *prometheus.CounterVec
}

// NewMetrics Factory method to create a new metrics collector.
func NewMetrics(info InstanceInfo) Metrics {
	m := &metrics{}

	m.registry = prometheus.NewRegistry()
	options := collectors.ProcessCollectorOpts{
		Namespace: MetricsNamespace,
	}
	m.registry.MustRegister(collectors.NewProcessCollector(options))
	m.registry.MustRegister(collectors.NewGoCollector())

	additionalLabels := map[string]string{}
	if info.InstallationID != "" {
		additionalLabels[MetricsCloudInstallationLabel] = info.InstallationID
	}

	m.pluginStartTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemSystem,
		Name:        "plugin_start_timestamp_seconds",
		Help:        "The time the plugin started.",
		ConstLabels: additionalLabels,
	})
	m.pluginStartTime.SetToCurrentTime()
	m.registry.MustRegister(m.pluginStartTime)

	m.pluginInfo = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: MetricsNamespace,
		Subsystem: MetricsSubsystemSystem,
		Name:      "plugin_info",
		Help:      "The plugin version.",
		ConstLabels: map[string]string{
			MetricsCloudInstallationLabel: info.InstallationID,
			MetricsVersionLabel:           info.PluginVersion,
		},
	})
	m.pluginInfo.Set(1)
	m.registry.MustRegister(m.pluginInfo)

	m.goroutineFailuresTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemSystem,
		Name:        "plugin_goroutine_failures_total",
		Help:        "The total number of times a goroutine has failed.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.goroutineFailuresTotal)

	m.apiTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   MetricsNamespace,
			Subsystem:   MetricsSubsystemAPI,
			Name:        "time_seconds",
			Help:        "Time to execute the api handler",
			ConstLabels: additionalLabels,
		},
		[]string{"handler", "method", "status_code"},
	)
	m.registry.MustRegister(m.apiTime)

	m.httpRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemHTTP,
		Name:        "requests_total",
		Help:        "The total number of http API requests.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.httpRequestsTotal)

	m.httpErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemHTTP,
		Name:        "errors_total",
		Help:        "The total number of http API errors.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.httpErrorsTotal)

	m.oAuthTokenInvalidatedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemMSGraph,
		Name:        "oauth_token_invalidated_total",
		Help:        "The total number of times an oAuth token has been invalidated.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.oAuthTokenInvalidatedTotal)

	m.changeEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "change_events_total",
		Help:        "The total number of MS Teams change events processed.",
		ConstLabels: additionalLabels,
	}, []string{"change_type", "discarded_reason"})
	m.registry.MustRegister(m.changeEventsTotal)

	m.lifecycleEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "lifecycle_events_total",
		Help:        "The total number of MS Teams lifecycle events received.",
		ConstLabels: additionalLabels,
	}, []string{"event_type", "discarded_reason"})
	m.registry.MustRegister(m.lifecycleEventsTotal)

	m.messagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "messages_total",
		Help:        "The total number of messages for different actions and sources",
		ConstLabels: additionalLabels,
	}, []string{"action", "source", "is_direct"})
	m.registry.MustRegister(m.messagesTotal)

	m.messageDelayTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "message_delay_seconds",
		Help:        "The delay between a message event across platforms",
		ConstLabels: additionalLabels,
	}, []string{"action", "source", "is_direct"})
	m.registry.MustRegister(m.messageDelayTime)

	m.reactionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "reactions_total",
		Help:        "The total number of reactions for different actions and sources",
		ConstLabels: additionalLabels,
	}, []string{"action", "source", "is_direct"})
	m.registry.MustRegister(m.reactionsTotal)

	m.filesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "files_total",
		Help:        "The total number of files for different actions and sources",
		ConstLabels: additionalLabels,
	}, []string{"action", "source", "is_direct", "discarded_reason"})
	m.registry.MustRegister(m.filesTotal)

	m.subscriptionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "subscriptions_total",
		Help:        "The total number of connected, reconnected and refreshed subscriptions.",
		ConstLabels: additionalLabels,
	}, []string{"action"})
	m.registry.MustRegister(m.subscriptionsTotal)

	m.connectedUsers = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "connected_users",
		Help:        "The total number of users connected to MS Teams users.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.connectedUsers)

	m.connectedUsersLimit = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "connected_users_limit",
		Help:        "The maximum number of users allowed to connect.",
		ConstLabels: additionalLabels,
	})
	m.connectedUsersLimit.Set(float64(info.ConnectedUsersLimit))
	m.registry.MustRegister(m.connectedUsersLimit)

	m.pendingInvites = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "pending_invites",
		Help:        "The total number of users with pending connection invites.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.pendingInvites)

	m.pendingInvitesLimit = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "pending_invites_limit",
		Help:        "The maximum number of pending connection invites.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.pendingInvitesLimit)

	m.whitelistedUsers = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "whitelisted_users",
		Help:        "The total number of users whitelisted for connection invites or new connections.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.whitelistedUsers)

	m.linkedChannels = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "linked_channels",
		Help:        "The total number of linked channels to MS Teams.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.linkedChannels)

	m.activeUsersReceiving = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "active_users_receiving",
		Help:        "The number of users who have received messages in the last week.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.activeUsersReceiving)

	m.changeEventQueueCapacity = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "change_event_queue_capacity",
		Help:        "The capacity of the change event queue.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.changeEventQueueCapacity)

	m.changeEventQueueLength = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "change_event_queue_length",
		Help:        "The length of the change event queue.",
		ConstLabels: additionalLabels,
	}, []string{"change_type"})
	m.registry.MustRegister(m.changeEventQueueLength)

	m.changeEventQueueRejectedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "change_event_queue_rejected_total",
		Help:        "The total number of change events rejected due to the activity queue size being full.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.changeEventQueueRejectedTotal)

	m.msGraphClientTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   MetricsNamespace,
			Subsystem:   MetricsSubsystemMSGraph,
			Name:        "client_time_seconds",
			Help:        "Time to execute the client methods",
			ConstLabels: additionalLabels,
		},
		[]string{"method", "success", "status_code"},
	)
	m.registry.MustRegister(m.msGraphClientTime)

	m.storeTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemDB,
		Name:        "store_time_seconds",
		Help:        "Time to execute the store method",
		ConstLabels: additionalLabels,
	}, []string{"method", "success"})
	m.registry.MustRegister(m.storeTime)

	m.activeWorkersTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "active_workers_total",
		Help:        "The number of active workers.",
		ConstLabels: additionalLabels,
	}, []string{"worker"})
	m.registry.MustRegister(m.activeWorkersTotal)

	m.clientSecretEndDateTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemMSGraph,
		Name:        "client_secret_end_date_timestamp_seconds",
		Help:        "The time the configured application credential expires.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.clientSecretEndDateTime)

	m.workersTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "workers_time_seconds",
		Help:        "Time to execute various workers.",
		ConstLabels: additionalLabels,
	}, []string{"worker"})
	m.registry.MustRegister(m.workersTime)

	m.notificationsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "notifications_total",
		Help:        "The total number of chat notifications delivered.",
		ConstLabels: additionalLabels,
	}, []string{"is_group_chat", "has_attachments"})
	m.registry.MustRegister(m.notificationsTotal)

	return m
}

func (m *metrics) GetRegistry() *prometheus.Registry {
	return m.registry
}

func (m *metrics) ObserveGoroutineFailure() {
	if m != nil {
		m.goroutineFailuresTotal.Inc()
	}
}

func (m *metrics) ObserveAPIEndpointDuration(handler, method, statusCode string, elapsed float64) {
	if m != nil {
		m.apiTime.With(prometheus.Labels{"handler": handler, "method": method, "status_code": statusCode}).Observe(elapsed)
	}
}

// START CONNECT FLOW METRICS

func (m *metrics) ObserveConnectedUsers(count int64) {
	if m != nil {
		m.connectedUsers.Set(float64(count))
	}
}

func (m *metrics) ObserveConnectedUsersLimit(limit int64) {
	if m != nil {
		m.connectedUsersLimit.Set(float64(limit))
	}
}

func (m *metrics) ObservePendingInvites(count int64) {
	if m != nil {
		m.pendingInvites.Set(float64(count))
	}
}

func (m *metrics) ObservePendingInvitesLimit(count int64) {
	if m != nil {
		m.pendingInvitesLimit.Set(float64(count))
	}
}

func (m *metrics) ObserveWhitelistedUsers(count int64) {
	if m != nil {
		m.whitelistedUsers.Set(float64(count))
	}
}

// END CONNECT FLOW METRICS

func (m *metrics) ObserveChangeEvent(changeType string, discardedReason string) {
	if m != nil {
		m.changeEventsTotal.With(prometheus.Labels{"change_type": changeType, "discarded_reason": discardedReason}).Inc()
	}
}

func (m *metrics) ObserveLifecycleEvent(eventType string, discardedReason string) {
	if m != nil {
		m.lifecycleEventsTotal.With(prometheus.Labels{"event_type": eventType, "discarded_reason": discardedReason}).Inc()
	}
}

func (m *metrics) ObserveMessage(action, source string, isDirectOrGroupMessage bool) {
	if m != nil {
		m.messagesTotal.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectOrGroupMessage)}).Inc()
	}
}

func (m *metrics) ObserveMessageDelay(action, source string, isDirectOrGroupMessage bool, delay time.Duration) {
	if m != nil {
		m.messageDelayTime.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectOrGroupMessage)}).Observe(delay.Seconds())
	}
}

func (m *metrics) ObserveReaction(action, source string, isDirectOrGroupMessage bool) {
	if m != nil {
		m.reactionsTotal.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectOrGroupMessage)}).Inc()
	}
}

func (m *metrics) ObserveFiles(action, source, discardedReason string, isDirectOrGroupMessage bool, count int64) {
	if m != nil {
		m.filesTotal.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectOrGroupMessage), "discarded_reason": discardedReason}).Add(float64(count))
	}
}

func (m *metrics) ObserveFile(action, source, discardedReason string, isDirectOrGroupMessage bool) {
	if m != nil {
		m.filesTotal.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectOrGroupMessage), "discarded_reason": discardedReason}).Inc()
	}
}

func (m *metrics) ObserveSubscription(action string) {
	if m != nil {
		m.subscriptionsTotal.With(prometheus.Labels{"action": action}).Inc()
	}
}

func (m *metrics) ObserveLinkedChannels(count int64) {
	if m != nil {
		m.linkedChannels.Set(float64(count))
	}
}

func (m *metrics) ObserveActiveUsersReceiving(count int64) {
	if m != nil {
		m.activeUsersReceiving.Set(float64(count))
	}
}

func (m *metrics) IncrementHTTPRequests() {
	if m != nil {
		m.httpRequestsTotal.Inc()
	}
}

func (m *metrics) IncrementHTTPErrors() {
	if m != nil {
		m.httpErrorsTotal.Inc()
	}
}

func (m *metrics) ObserveOAuthTokenInvalidated() {
	if m != nil {
		m.oAuthTokenInvalidatedTotal.Inc()
	}
}

func (m *metrics) ObserveChangeEventQueueRejected() {
	if m != nil {
		m.changeEventQueueRejectedTotal.Inc()
	}
}

func (m *metrics) ObserveChangeEventQueueCapacity(count int64) {
	if m != nil {
		m.changeEventQueueCapacity.Set(float64(count))
	}
}

func (m *metrics) IncrementChangeEventQueueLength(changeType string) {
	if m != nil {
		m.changeEventQueueLength.With(prometheus.Labels{"change_type": changeType}).Inc()
	}
}

func (m *metrics) DecrementChangeEventQueueLength(changeType string) {
	if m != nil {
		m.changeEventQueueLength.With(prometheus.Labels{"change_type": changeType}).Dec()
	}
}

func (m *metrics) ObserveMSGraphClientMethodDuration(method, success, statusCode string, elapsed float64) {
	if m != nil {
		m.msGraphClientTime.With(prometheus.Labels{"method": method, "success": success, "status_code": statusCode}).Observe(elapsed)
	}
}

func (m *metrics) ObserveStoreMethodDuration(method, success string, elapsed float64) {
	if m != nil {
		m.storeTime.With(prometheus.Labels{"method": method, "success": success}).Observe(elapsed)
	}
}

func (m *metrics) IncrementActiveWorkers(worker string) {
	if m != nil {
		m.activeWorkersTotal.With(prometheus.Labels{"worker": worker}).Inc()
	}
}

func (m *metrics) DecrementActiveWorkers(worker string) {
	if m != nil {
		m.activeWorkersTotal.With(prometheus.Labels{"worker": worker}).Dec()
	}
}

func (m *metrics) ObserveWorkerDuration(worker string, elapsed float64) {
	if m != nil {
		m.workersTime.With(prometheus.Labels{"worker": worker}).Observe(elapsed)
	}
}

func (m *metrics) ObserveClientSecretEndDateTime(expireDate time.Time) {
	if m != nil {
		if expireDate.IsZero() {
			m.clientSecretEndDateTime.Set(0)
		} else {
			m.clientSecretEndDateTime.Set(float64(expireDate.UnixNano()) / 1e9)
		}
	}
}

func (m *metrics) ObserveSyncMsgPostDelay(action string, delayMillis int64) {
	if m != nil {
		m.syncMsgPostDelayTime.With(prometheus.Labels{"action": action}).Observe(float64(delayMillis) / 1000)
	}
}

func (m *metrics) ObserveSyncMsgReactionDelay(action string, delayMillis int64) {
	if m != nil {
		m.syncMsgReactionDelayTime.With(prometheus.Labels{"action": action}).Observe(float64(delayMillis) / 1000)
	}
}

func (m *metrics) ObserveSyncMsgFileDelay(action string, delayMillis int64) {
	if m != nil {
		m.syncMsgFileDelayTime.With(prometheus.Labels{"action": action}).Observe(float64(delayMillis) / 1000)
	}
}

// ObserveWorker is a helper routine that abstracts tracking active workers and duration, returning
// a callback to invoke when the worker is done executing.
func (m *metrics) ObserveWorker(worker string) func() {
	if m != nil {
		m.IncrementActiveWorkers(worker)
		start := time.Now()

		return func() {
			m.DecrementActiveWorkers(worker)
			elapsed := float64(time.Since(start)) / float64(time.Second)
			m.ObserveWorkerDuration(worker, elapsed)
		}
	}

	return func() {}
}

func (m *metrics) ObserveNotification(isGroupChat, hasAttachments bool) {
	if m != nil {
		m.notificationsTotal.With(prometheus.Labels{
			"is_group_chat":   strconv.FormatBool(isGroupChat),
			"has_attachments": strconv.FormatBool(hasAttachments),
		}).Inc()
	}
}
