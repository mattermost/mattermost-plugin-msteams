package monitor

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-api/cluster"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

const monitoringSystemJobName = "monitoring_system"

type Monitor struct {
	client           msteams.Client
	store            store.Store
	api              plugin.API
	job              *cluster.Job
	baseURL          string
	webhookSecret    string
	useEvaluationAPI bool
}

func New(client msteams.Client, store store.Store, api plugin.API, baseURL string, webhookSecret string, useEvaluationAPI bool) *Monitor {
	return &Monitor{
		client:           client,
		store:            store,
		api:              api,
		baseURL:          baseURL,
		webhookSecret:    webhookSecret,
		useEvaluationAPI: useEvaluationAPI,
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
	return m.store.SetJobStatus(monitoringSystemJobName, false)
}

func (m *Monitor) RunMonitoringSystemJob() {
	defer func() {
		if sErr := m.store.SetJobStatus(monitoringSystemJobName, false); sErr != nil {
			m.api.LogDebug("Failed to set monitoring job running status to false.")
		}
	}()

	isStatusUpdated, sErr := m.store.CompareAndSetJobStatus(monitoringSystemJobName, false, true)
	if sErr != nil {
		m.api.LogError("Something went wrong while fetching monitoring job status", "Error", sErr.Error())
		return
	}

	if !isStatusUpdated {
		m.api.LogDebug("Monitoring job already running")
		return
	}

	m.api.LogDebug("Running the Monitoring System Job")
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
	msteamsSubscriptionsMap, allChatsSubscription, err := m.GetMSTeamsSubscriptionsMap()
	if err != nil {
		m.api.LogError("Unable to fetch subscriptions for MS Teams", "error", err)
		return
	}

	m.checkGlobalSubscriptions(msteamsSubscriptionsMap, allChatsSubscription)
	m.checkChannelsSubscriptions(msteamsSubscriptionsMap, nil)
}

func (m *Monitor) GetMSTeamsSubscriptionsMap() (map[string]*msteams.Subscription, *msteams.Subscription, error) {
	msteamsSubscriptions, err := m.client.ListSubscriptions()
	if err != nil {
		m.api.LogError("Unable to list MS Teams subscriptions", "error", err)
		return nil, nil, err
	}

	var allChatsSubscription *msteams.Subscription
	msteamsSubscriptionsMap := make(map[string]*msteams.Subscription)
	for _, msteamsSubscription := range msteamsSubscriptions {
		msteamsSubscriptionsMap[msteamsSubscription.ID] = msteamsSubscription
		if strings.Contains(msteamsSubscription.Resource, "chats/getAllMessages") {
			allChatsSubscription = msteamsSubscription
		}
	}

	return msteamsSubscriptionsMap, allChatsSubscription, nil
}
