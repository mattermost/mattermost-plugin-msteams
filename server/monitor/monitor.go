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
	m.api.LogInfo("Starting the msteams sync monitoring system")

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
	return m.store.SetJobStatus(monitoringSystemJobName, false)
}

func (m *Monitor) RunMonitoringSystemJob() {
	defer func() {
		if r := recover(); r != nil {
			m.metrics.ObserveGoroutineFailure()
			m.api.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	defer func() {
		if sErr := m.store.SetJobStatus(monitoringSystemJobName, false); sErr != nil {
			m.api.LogError("Failed to set monitoring job running status to false.")
		}
	}()

	isStatusUpdated, sErr := m.store.CompareAndSetJobStatus(monitoringSystemJobName, false, true)
	if sErr != nil {
		m.api.LogError("Something went wrong while fetching monitoring job status", "error", sErr.Error())
		return
	}

	if !isStatusUpdated {
		return
	}

	m.api.LogInfo("Running the Monitoring System Job")
	m.check()
}

func (m *Monitor) Stop() {
	if m.job != nil {
		if err := m.job.Close(); err != nil {
			m.api.LogError("Failed to close monitoring system background job", "error", err)
		}
	}
}

func (m *Monitor) check() {
	done := m.metrics.ObserveWorker(metrics.WorkerMonitor)
	defer done()

	msteamsSubscriptionsMap, allChatsSubscription, err := m.GetMSTeamsSubscriptionsMap()
	if err != nil {
		m.api.LogWarn("Unable to fetch subscriptions from MS Teams", "error", err.Error())
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
