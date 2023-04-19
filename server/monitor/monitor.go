package monitor

import (
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

type Monitor struct {
	client        msteams.Client
	store         store.Store
	api           plugin.API
	baseURL       string
	webhookSecret string
	evaluationAPI bool
}

func New() *Monitor {
	return &Monitor{}
}

func (m *Monitor) Start() {
}

func (m *Monitor) Stop() {
}

func (m *Monitor) check() {
	m.checkChannelsSubscriptions()
	m.checkChatsSubscriptions()
}
