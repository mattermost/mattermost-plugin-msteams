package monitor

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/store"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
)

const monitoringSystemJobName = "monitoring_system"

// Monitor is a job that creates and maintains chat and channel subscriptions.
//
// While the job is started on all plugin instances in a cluster, only one instance will actually
// do the required effort, falling over seamlessly as needed.
type Monitor struct {
	client           msteams.Client
	store            store.Store
	api              plugin.API
	metrics          metrics.Metrics
	job              *cluster.Job
	baseURL          string
	webhookSecret    string
	certificate      string
	useEvaluationAPI bool
	startupTime      time.Time
}

// New creates a new instance of the Monitor job.
func New(client msteams.Client, store store.Store, api plugin.API, metrics metrics.Metrics, baseURL string, webhookSecret string, useEvaluationAPI bool, certificate string) *Monitor {
	return &Monitor{
		client:           client,
		store:            store,
		api:              api,
		metrics:          metrics,
		baseURL:          baseURL,
		webhookSecret:    webhookSecret,
		useEvaluationAPI: useEvaluationAPI,
		certificate:      certificate,
		startupTime:      time.Now(),
	}
}

// Start starts running the Monitor job.
func (m *Monitor) Start() error {
	m.api.LogInfo("Starting the msteams sync monitoring system")

	// Close the previous background job if exists.
	m.Stop()

	job, jobErr := cluster.Schedule(
		m.api,
		monitoringSystemJobName,
		cluster.MakeWaitForRoundedInterval(1*time.Minute),
		m.runMonitoringSystemJob,
	)
	if jobErr != nil {
		return fmt.Errorf("error in scheduling the monitoring system job. error: %w", jobErr)
	}

	m.job = job

	return nil
}

// Stop stops running the Monitor job.
func (m *Monitor) Stop() {
	if m.job != nil {
		if err := m.job.Close(); err != nil {
			m.api.LogError("Failed to close monitoring system background job", "error", err)
		}
	}
}

// runMonitoringSystemJob is a callback to trigger the business logic of the Monitor job, being run
// automatically by the job subsystem.
func (m *Monitor) runMonitoringSystemJob() {
	// Wait at least one minute after startup before starting the monitoring job.
	if time.Since(m.startupTime) < 1*time.Minute {
		m.api.LogInfo("Delaying the Monitoring System Job until at least 1 minute after startup")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			m.metrics.ObserveGoroutineFailure()
			m.api.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	m.api.LogInfo("Running the Monitoring System Job")

	done := m.metrics.ObserveWorker(metrics.WorkerMonitor)
	defer done()

	msteamsSubscriptionsMap, allChatsSubscription, err := m.getMSTeamsSubscriptionsMap()
	if err != nil {
		m.api.LogError("Unable to fetch subscriptions from MS Teams", "error", err.Error())
		return
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.checkChannelsSubscriptions(msteamsSubscriptionsMap)
	}()

	m.checkGlobalChatsSubscription(msteamsSubscriptionsMap, allChatsSubscription)

	wg.Wait()
}
