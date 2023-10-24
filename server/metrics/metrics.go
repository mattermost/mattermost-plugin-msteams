package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

const (
	MetricsNamespace       = "msteams_connect"
	MetricsSubsystemSystem = "system"
	MetricsSubsystemApp    = "app"
	MetricsSubsystemHTTP   = "http"
	MetricsSubsystemAPI    = "api"
	MetricsSubsystemEvents = "events"

	MetricsCloudInstallationLabel = "installationId"
)

type Metrics interface {
	ObserveAPIEndpointDuration(handler, method, statusCode string, elapsed float64)
	ObserveConnectedUsersTotal(count int64)
	ObserveChangeEventTotal(changeType string)
	ObserveProcessedChangeEventTotal(changeType string, discardedReason string)
	ObserveLifecycleEventTotal(lifecycleEventType string)
	ObserveMessagesCount(action, source string, isDirectMessage bool)
	ObserveReactionsCount(action, source string, isDirectMessage bool)
	ObserveFilesCount(action, source, discardedReason string, isDirectMessage bool, count int64)
	ObserveFileCount(action, source, discardedReason string, isDirectMessage bool)
	ObserveSyntheticUsersTotal(count int64)
	ObserveLinkedChannelsTotal(count int64)
	IncrementHTTPRequests()
	IncrementHTTPErrors()
	GetRegistry() *prometheus.Registry
	ObserveChangeEventQueueCapacity(count int64)
	IncrementChangeEventQueueLength(changeType string)
	DecrementChangeEventQueueLength(changeType string)
}

type InstanceInfo struct {
	InstallationID string
}

// metrics used to instrumentate metrics in prometheus.
type metrics struct {
	registry *prometheus.Registry

	pluginStartTime prometheus.Gauge

	apiTime *prometheus.HistogramVec

	httpRequestsTotal prometheus.Counter
	httpErrorsTotal   prometheus.Counter

	changeEventTotal          *prometheus.CounterVec
	lifecycleEventTotal       *prometheus.CounterVec
	processedChangeEventTotal *prometheus.CounterVec
	messagesCount             *prometheus.CounterVec
	reactionsCount            *prometheus.CounterVec
	filesCount                *prometheus.CounterVec

	connectedUsersTotal prometheus.Gauge
	syntheticUsersTotal prometheus.Gauge
	linkedChannelsTotal prometheus.Gauge

	changeEventQueueCapacity prometheus.Gauge
	changeEventQueueLength   *prometheus.GaugeVec
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
		Name:        "plugin_start_time",
		Help:        "The time the plugin started.",
		ConstLabels: additionalLabels,
	})
	m.pluginStartTime.SetToCurrentTime()
	m.registry.MustRegister(m.pluginStartTime)

	m.apiTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   MetricsNamespace,
			Subsystem:   MetricsSubsystemAPI,
			Name:        "time",
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

	m.changeEventTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "change_event_total",
		Help:        "The total number of MS Teams change events received.",
		ConstLabels: additionalLabels,
	}, []string{"change_type"})
	m.registry.MustRegister(m.changeEventTotal)

	m.processedChangeEventTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "processed_change_event_total",
		Help:        "The total number of MS Teams change events processed.",
		ConstLabels: additionalLabels,
	}, []string{"change_type", "discarded_reason"})
	m.registry.MustRegister(m.processedChangeEventTotal)

	m.lifecycleEventTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "lifecycle_event_total",
		Help:        "The total number of MS Teams lifecycle events received.",
		ConstLabels: additionalLabels,
	}, []string{"event_type"})
	m.registry.MustRegister(m.lifecycleEventTotal)

	m.messagesCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "messages_count",
		Help:        "The total number of messages for different actions and sources",
		ConstLabels: additionalLabels,
	}, []string{"action", "source", "is_direct"})
	m.registry.MustRegister(m.messagesCount)

	m.reactionsCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "reactions_count",
		Help:        "The total number of reactions for different actions and sources",
		ConstLabels: additionalLabels,
	}, []string{"action", "source", "is_direct"})
	m.registry.MustRegister(m.reactionsCount)

	m.filesCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemEvents,
		Name:        "files_count",
		Help:        "The total number of files for different actions and sources",
		ConstLabels: additionalLabels,
	}, []string{"action", "source", "is_direct", "discarded_reason"})
	m.registry.MustRegister(m.filesCount)

	m.connectedUsersTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "connected_users_total",
		Help:        "The total number of Mattermost users connected to MS Teams users.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.connectedUsersTotal)

	m.syntheticUsersTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "synthetic_users_total",
		Help:        "The total number of synthetic users.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.syntheticUsersTotal)

	m.linkedChannelsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   MetricsNamespace,
		Subsystem:   MetricsSubsystemApp,
		Name:        "linked_channels_total",
		Help:        "The total number of linked channels to MS Teams.",
		ConstLabels: additionalLabels,
	})
	m.registry.MustRegister(m.linkedChannelsTotal)

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

func (m *metrics) ObserveConnectedUsersTotal(count int64) {
	if m != nil {
		m.connectedUsersTotal.Set(float64(count))
	}
}

func (m *metrics) ObserveChangeEventTotal(changeType string) {
	if m != nil {
		m.changeEventTotal.With(prometheus.Labels{"change_type": changeType}).Inc()
	}
}

func (m *metrics) ObserveProcessedChangeEventTotal(changeType string, discardedReason string) {
	if m != nil {
		m.processedChangeEventTotal.With(prometheus.Labels{"change_type": changeType, "discarded_reason": discardedReason}).Inc()
	}
}

func (m *metrics) ObserveLifecycleEventTotal(lifecycleEventType string) {
	if m != nil {
		m.lifecycleEventTotal.With(prometheus.Labels{"event_type": lifecycleEventType}).Inc()
	}
}

func (m *metrics) ObserveMessagesCount(action, source string, isDirectMessage bool) {
	if m != nil {
		m.messagesCount.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectMessage)}).Inc()
	}
}

func (m *metrics) ObserveReactionsCount(action, source string, isDirectMessage bool) {
	if m != nil {
		m.reactionsCount.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectMessage)}).Inc()
	}
}

func (m *metrics) ObserveFilesCount(action, source, discardedReason string, isDirectMessage bool, count int64) {
	if m != nil {
		m.filesCount.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectMessage), "discarded_reason": discardedReason}).Add(float64(count))
	}
}

func (m *metrics) ObserveFileCount(action, source, discardedReason string, isDirectMessage bool) {
	if m != nil {
		m.filesCount.With(prometheus.Labels{"action": action, "source": source, "is_direct": strconv.FormatBool(isDirectMessage), "discarded_reason": discardedReason}).Inc()
	}
}

func (m *metrics) ObserveSyntheticUsersTotal(count int64) {
	if m != nil {
		m.syntheticUsersTotal.Set(float64(count))
	}
}

func (m *metrics) ObserveLinkedChannelsTotal(count int64) {
	if m != nil {
		m.linkedChannelsTotal.Set(float64(count))
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
