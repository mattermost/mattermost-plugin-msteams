package monitor

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
)

const monitoringSystemJobName = "monitoring_system"

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
}

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
	}
}

func (m *Monitor) Start() error {
	m.api.LogDebug("Starting the msteams sync monitoring system")

	// Close the previous background job if exists.
	m.Stop()

	job, jobErr := cluster.Schedule(
		m.api,
		monitoringSystemJobName,
		cluster.MakeWaitForRoundedInterval(1*time.Minute),
		m.RunMonitoringSystemJob,
	)
	if jobErr != nil {
		return fmt.Errorf("error in scheduling the monitoring system job. error: %w", jobErr)
	}

	m.job = job

	return nil
}

func (m *Monitor) Stop() {
	if m.job != nil {
		if err := m.job.Close(); err != nil {
			m.api.LogError("Failed to close monitoring system background job", "error", err)
		}
	}
}

func (m *Monitor) RunMonitoringSystemJob() {
	defer func() {
		if r := recover(); r != nil {
			m.metrics.ObserveGoroutineFailure()
			m.api.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	m.api.LogDebug("Running the Monitoring System Job")

	done := m.metrics.ObserveWorker(metrics.WorkerMonitor)
	defer done()

	msteamsSubscriptionsMap, allChatsSubscription, err := m.GetMSTeamsSubscriptionsMap()
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

	m.checkGlobalSubscriptions(msteamsSubscriptionsMap, allChatsSubscription)

	wg.Wait()
}
