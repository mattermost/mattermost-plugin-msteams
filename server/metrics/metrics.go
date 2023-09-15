package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

const (
	MetricsNamespace       = "msteams_connect"
	MetricsSubsystemApp    = "app"
	MetricsSubsystemHTTP   = "http"
	MetricsSubsystemAPI    = "api"
	MetricsSubsystemEvents = "events"

	MetricsCloudInstallationLabel = "installationId"
)

type InstanceInfo struct {
	InstallationID string
}

// Metrics used to instrumentate metrics in prometheus.
type Metrics struct {
	registry *prometheus.Registry

	apiTime *prometheus.HistogramVec

	httpRequestsTotal prometheus.Counter
	httpErrorsTotal   prometheus.Counter

	changeEventTotal          *prometheus.CounterVec
	lifecycleEventTotal       *prometheus.CounterVec
	processedChangeEventTotal *prometheus.CounterVec

	connectedUsersTotal prometheus.Gauge
	syntheticUsersTotal prometheus.Gauge
	linkedChannelsTotal prometheus.Gauge
}

// NewMetrics Factory method to create a new metrics collector.
func NewMetrics(info InstanceInfo) *Metrics {
	m := &Metrics{}

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
		Name:        "discared_change_event_total",
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
	}, []string{"lifecycle_event_type"})
	m.registry.MustRegister(m.lifecycleEventTotal)

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

	return m
}

func (m *Metrics) ObserveAPIEndpointDuration(handler, method, statusCode string, elapsed float64) {
	if m != nil {
		m.apiTime.With(prometheus.Labels{"handler": handler, "method": method, "status_code": statusCode}).Observe(elapsed)
	}
}

func (m *Metrics) ObserveConnectedUsersTotal(count int64) {
	if m != nil {
		m.connectedUsersTotal.Set(float64(count))
	}
}

func (m *Metrics) ObserveChangeEventTotal(changeType string) {
	if m != nil {
		m.changeEventTotal.With(prometheus.Labels{"change_type": changeType}).Inc()
	}
}

func (m *Metrics) ObserveProcessedChangeEventTotal(changeType string, discardedReason string) {
	if m != nil {
		m.processedChangeEventTotal.With(prometheus.Labels{"change_type": changeType, "discarded_reason": discardedReason}).Inc()
	}
}

func (m *Metrics) ObserveLifecycleEventTotal(lifecycleEventType string) {
	if m != nil {
		m.lifecycleEventTotal.With(prometheus.Labels{"lifecycle_event_type": lifecycleEventType}).Inc()
	}
}

func (m *Metrics) ObserveSyntheticUsersTotal(count int64) {
	if m != nil {
		m.syntheticUsersTotal.Set(float64(count))
	}
}

func (m *Metrics) ObserveLinkedChannelsTotal(count int64) {
	if m != nil {
		m.linkedChannelsTotal.Set(float64(count))
	}
}

func (m *Metrics) IncrementHTTPRequests() {
	if m != nil {
		m.httpRequestsTotal.Inc()
	}
}

func (m *Metrics) IncrementHTTPErrors() {
	if m != nil {
		m.httpErrorsTotal.Inc()
	}
}
