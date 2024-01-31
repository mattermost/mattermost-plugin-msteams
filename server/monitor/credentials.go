package monitor

import (
	"time"
)

func (m *Monitor) checkCredentials(force bool) {
	if !force && (time.Now().Hour() != 0 || time.Now().Minute() != 0) {
		return
	}

	if m.applicationID == "" {
		return
	}

	credentials, err := m.client.GetAppCredentials(m.applicationID)
	if err != nil {
		m.api.LogWarn("Error getting client secret expire date", "error", err.Error())
		return
	}

	for _, credential := range credentials {
		m.metrics.ObserveClientSecretEndDateTime(credential.ID, credential.EndDateTime)
	}
}
