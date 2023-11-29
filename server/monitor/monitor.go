package monitor

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/handlers"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
)

const monitoringSystemJobName = "monitoring_system"

type Monitor struct {
	client           msteams.Client
	activityHandler  *handlers.ActivityHandler
	store            store.Store
	api              plugin.API
	metrics          metrics.Metrics
	job              *cluster.Job
	baseURL          string
	webhookSecret    string
	certificate      string
	useEvaluationAPI bool
	firstTime        bool
}

func New(client msteams.Client, activityHandler *handlers.ActivityHandler, store store.Store, api plugin.API, metrics metrics.Metrics, baseURL string, webhookSecret string, useEvaluationAPI bool, certificate string) *Monitor {
	return &Monitor{
		client:           client,
		store:            store,
		activityHandler:  activityHandler,
		api:              api,
		metrics:          metrics,
		baseURL:          baseURL,
		webhookSecret:    webhookSecret,
		useEvaluationAPI: useEvaluationAPI,
		certificate:      certificate,
		firstTime:        true,
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
		if r := recover(); r != nil {
			m.metrics.ObserveGoroutineFailure()
			m.api.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
		}
	}()

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
		m.api.LogError("Unable to fetch subscriptions from MS Teams", "error", err.Error())
		return
	}

	go func(firstTime bool) {
		m.checkChannelsSubscriptions(msteamsSubscriptionsMap, firstTime)
	}(m.firstTime)
	go m.checkGlobalSubscriptions(msteamsSubscriptionsMap, allChatsSubscription, m.firstTime)
	m.firstTime = false
}

func (m *Monitor) syncChannelSince(teamID, channelID string, syncSince time.Time) {
	messages, err := m.client.ListChannelMessages(teamID, channelID, syncSince)
	if err != nil {
		m.api.LogError("Unable to sync channel messages", "teamID", teamID, "channelID", channelID, "date", syncSince, "error", err)
	}
	for _, message := range messages {
		isCreation := false
		if message.CreateAt == message.LastUpdateAt {
			isCreation = true
		} else {
			post, err := m.store.GetPostInfoByMSTeamsID("", message.ID)
			if err != nil || post == nil {
				isCreation = false
			}
		}

		if isCreation {
			m.activityHandler.HandleCreatedActivity(message, clientmodels.ActivityIds{
				TeamID:    teamID,
				ChannelID: channelID,
				MessageID: message.ID,
				ReplyID:   message.ReplyToID,
			})
		} else {
			m.activityHandler.HandleUpdatedActivity(message, clientmodels.ActivityIds{
				TeamID:    teamID,
				ChannelID: channelID,
				MessageID: message.ID,
				ReplyID:   message.ReplyToID,
			})
		}
	}
}

func (m *Monitor) syncChatsSince(syncSince time.Time) {
	connectedUsers, err := m.store.GetConnectedUsers(0, 10000000)
	if err != nil {
		m.api.LogError("Unable to get the connected users: sync failed", "since", syncSince)
	} else {
		for _, user := range connectedUsers {
			messages, err := m.client.GetUserChatMessagesSince(user.TeamsUserID, syncSince, m.useEvaluationAPI)
			if err != nil {
				m.api.LogError("Unable to sync user messages", "userID", user.TeamsUserID, "date", syncSince, "error", err)
			}
			for _, message := range messages {
				isCreation := false
				if message.CreateAt == message.LastUpdateAt {
					isCreation = true
				} else {
					post, err := m.store.GetPostInfoByMSTeamsID(message.ChatID, message.ID)
					if err != nil || post == nil {
						isCreation = false
					}
				}

				if isCreation {
					m.activityHandler.HandleCreatedActivity(message, clientmodels.ActivityIds{
						ChatID:    message.ChatID,
						MessageID: message.ID,
						ReplyID:   message.ReplyToID,
					})
				} else {
					m.activityHandler.HandleUpdatedActivity(message, clientmodels.ActivityIds{
						ChatID:    message.ChatID,
						MessageID: message.ID,
						ReplyID:   message.ReplyToID,
					})
				}
			}
		}
	}
}
