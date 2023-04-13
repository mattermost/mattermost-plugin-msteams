package monitor

type Monitor struct{}

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

func (m *Monitor) checkChannelsSubscriptions() {
}

func (m *Monitor) checkChatsSubscriptions() {
}
