package monitor

import (
	"context"
	"errors"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

type Monitor struct {
	client           msteams.Client
	store            store.Store
	api              plugin.API
	ctx              context.Context
	cancel           context.CancelFunc
	baseURL          string
	webhookSecret    string
	certificate      string
	useEvaluationAPI bool
}

func New(client msteams.Client, store store.Store, api plugin.API, baseURL string, webhookSecret string, useEvaluationAPI bool, certificate string) *Monitor {
	return &Monitor{
		client:           client,
		store:            store,
		api:              api,
		baseURL:          baseURL,
		webhookSecret:    webhookSecret,
		useEvaluationAPI: useEvaluationAPI,
		certificate:      certificate,
	}
}

func (m *Monitor) Start() error {
	if m.ctx != nil {
		return errors.New("the monitor is already running")
	}

	m.api.LogDebug("Running the msteams sync monitoring system")
	ticker := time.NewTicker(1 * time.Minute)
	m.ctx, m.cancel = context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-ticker.C:
				m.check()
			case <-m.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
	return nil
}

func (m *Monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
		m.ctx = nil
		m.cancel = nil
	}
}

func (m *Monitor) check() {
	m.checkGlobalSubscriptions()
	m.checkChannelsSubscriptions()
	m.checkChatsSubscriptions()
}
