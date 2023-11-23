//go:generate mockery --name=Metrics
package metrics

import (
	"strconv"

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

	ActionSourceMSTeams     = "msteams"
	ActionSourceMattermost  = "mattermost"
	ActionCreated           = "created"
	ActionUpdated           = "updated"
	ActionDeleted           = "deleted"
	ReactionSetAction       = "set"
	ReactionUnsetAction     = "unset"
	SubscriptionRefreshed   = "refreshed"
	SubscriptionConnected   = "connected"
	SubscriptionReconnected = "reconnected"

	DiscardedReasonNone                      = ""
	DiscardedReasonUnableToGetMMData         = "unable_to_get_mm_data"
	DiscardedReasonUnableToUploadFileOnTeams = "unable_to_upload_file_on_teams"
	DiscardedReasonInvalidChangeType         = "invalid_change_type"
	DiscardedReasonIsBotUser                 = "is_bot_user"
	DiscardedReasonUnableToGetTeamsData      = "unable_to_get_teams_data"
	DiscardedReasonNotUserEvent              = "no_user_event"
	DiscardedReasonOther                     = "other"
	DiscardedReasonDirectMessagesDisabled    = "direct_messages_disabled"
	DiscardedReasonInactiveUser              = "inactive_user"
	DiscardedReasonDuplicatedPost            = "duplicated_post"
	DiscardedReasonAlreadyAppliedChange      = "already_applied_change"
	DiscardedReasonFileLimitReached          = "file_limit_reached"
	DiscardedReasonEmptyFileID               = "empty_file_id"
	DiscardedReasonMaxFileSizeExceeded       = "max_file_size_exceeded"
	DiscardedReasonExpiredSubscription       = "expired_subscription"
	DiscardedReasonInvalidWebhookSecret      = "invalid_webhook_secret"
	DiscardedReasonFailedSubscriptionCheck   = "failed_subscription_check"
	DiscardedReasonFailedToRefresh           = "failed_to_refresh"
)

type Metrics interface {
	ObserveAPIEndpointDuration(handler, method, statusCode string, elapsed float64)

	IncrementHTTPRequests()
	IncrementHTTPErrors()
	ObserveChangeEventQueueRejected()

	ObserveChangeEvent(changeType string, discardedReason string)
	ObserveLifecycleEvent(lifecycleEventType, discardedReason string)
	ObserveMessage(action, source string, isDirectMessage bool)
	ObserveReaction(action, source string, isDirectMessage bool)
	ObserveFiles(action, source, discardedReason string, isDirectMessage bool, count int64)
	ObserveFile(action, source, discardedReason string, isDirectMessage bool)
	ObserveConfirmedMessage(source string, isDirectMessage bool)
	ObserveSubscription(action string)

	ObserveConnectedUsers(count int64)
	ObserveSyntheticUsers(count int64)
	ObserveLinkedChannels(count int64)
	ObserveUpstreamUsers(count int64)
	ObserveChangeEventQueueCapacity(count int64)
	IncrementChangeEventQueueLength(changeType string)
	DecrementChangeEventQueueLength(changeType string)

	ObserveMSGraphClientMethodDuration(method, success string, elapsed float64)
	ObserveStoreMethodDuration(method, success string, elapsed float64)

	GetRegistry() *prometheus.Registry
}

type InstanceInfo struct {
	InstallationID string
}

// metrics used to instrumentate metrics in prometheus.
type metrics struct {
	registry *prometheus.Registry

	pluginStartTime prometheus.Gauge

	apiTime *prometheus.HistogramVec

	msGraphClientTime *prometheus.HistogramVec

	httpRequestsTotal prometheus.Counter
	httpErrorsTotal   prometheus.Counter

	lifecycleEventsTotal   *prometheus.CounterVec
	changeEventsTotal      *prometheus.CounterVec
	messagesTotal          *prometheus.CounterVec
	reactionsTotal         *prometheus.CounterVec
	filesTotal             *prometheus.CounterVec
	messagesConfirmedTotal *prometheus.CounterVec
	subscriptionsTotal     *prometheus.CounterVec

	connectedUsers prometheus.Gauge
	syntheticUsers prometheus.Gauge
	linkedChannels prometheus.Gauge
	upstreamUsers  prometheus.Gauge

	changeEventQueueCapacity      prometheus.Gauge
	changeEventQueueLength        *prometheus.GaugeVec
	changeEventQueueRejectedTotal prometheus.Counter

	storeTime *prometheus.HistogramVec
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

	// TODO: Why?
	m.messagesConfirmedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "messages_confirmed_total",
		Help:        "The total number of messages confirmed to be sent from Mattermost to MS Teams and vice versa.",
		ConstLabels: additionalLabels,
	}, []string{"source", "is_direct"})
	m.registry.MustRegister(m.messagesConfirmedTotal)

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
		Help:        "The total number of Mattermost users connected to MS Teams users.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.connectedUsers)

	m.syntheticUsers = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "synthetic_users",
		Help:        "The total number of synthetic users.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.syntheticUsers)

	m.linkedChannels = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "linked_channels",
		Help:        "The total number of linked channels to MS Teams.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.linkedChannels)

	m.upstreamUsers = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "upstream_users",
		Help:        "The total number of upstream users.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.upstreamUsers)

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
		[]string{"method", "success"},
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

	return m
}

func (m *metrics) GetRegistry() *prometheus.Registry {
	return m.registry
}

func (m *metrics) ObserveAPIEndpointDuration(handler, method, statusCode string, elapsed float64) {
	if m != nil {
		m.apiTime.With(prometheus.Labels{"handler": handler, "method": method, "status_code": statusCode}).Observe(elapsed)
	}
}

func (m *metrics) ObserveConnectedUsers(count int64) {
	if m != nil {
		m.connectedUsers.Set(float64(count))
	}
}

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

func (m *metrics) ObserveMessage(action, source string, isDirectMessage bool) {
	if m != nil {
		m.messagesTotal.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectMessage)}).Inc()
	}
}

func (m *metrics) ObserveReaction(action, source string, isDirectMessage bool) {
	if m != nil {
		m.reactionsTotal.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectMessage)}).Inc()
	}
}

func (m *metrics) ObserveFiles(action, source, discardedReason string, isDirectMessage bool, count int64) {
	if m != nil {
		m.filesTotal.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectMessage), "discarded_reason": discardedReason}).Add(float64(count))
	}
}

func (m *metrics) ObserveFile(action, source, discardedReason string, isDirectMessage bool) {
	if m != nil {
		m.filesTotal.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectMessage), "discarded_reason": discardedReason}).Inc()
	}
}

func (m *metrics) ObserveConfirmedMessage(source string, isDirectMessage bool) {
	if m != nil {
		m.messagesConfirmedTotal.With(prometheus.Labels{"source": source, "is_direct": strconv.FormatBool(isDirectMessage)}).Inc()
	}
}

func (m *metrics) ObserveSubscription(action string) {
	if m != nil {
		m.subscriptionsTotal.With(prometheus.Labels{"action": action}).Inc()
	}
}

func (m *metrics) ObserveSyntheticUsers(count int64) {
	if m != nil {
		m.syntheticUsers.Set(float64(count))
	}
}

func (m *metrics) ObserveLinkedChannels(count int64) {
	if m != nil {
		m.linkedChannels.Set(float64(count))
	}
}
func (m *metrics) ObserveUpstreamUsers(count int64) {
	if m != nil {
		m.upstreamUsers.Set(float64(count))
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

func (m *metrics) ObserveMSGraphClientMethodDuration(method, success string, elapsed float64) {
	if m != nil {
		m.msGraphClientTime.With(prometheus.Labels{"method": method, "success": success}).Observe(elapsed)
	}
}

func (m *metrics) ObserveStoreMethodDuration(method, success string, elapsed float64) {
	if m != nil {
		m.storeTime.With(prometheus.Labels{"method": method, "success": success}).Observe(elapsed)
	}
}
