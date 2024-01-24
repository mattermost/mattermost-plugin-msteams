package monitor

import (
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

func (m *Monitor) checkCredentials(force bool) {
	if !force && (time.Now().Hour() != 0 || time.Now().Minute() != 0) {
		return
	}

	credentials, err := m.client.GetAppCredentials()
	if err != nil {
		m.api.LogDebug("Error getting client secret expire date", "error", err.Error())
		return
	}

	for _, credential := range credentials {
		m.metrics.ObserveClientSecretExpireDate(credential.ID, credential.ExpireDate)
		if credential.ExpireDate.Before(time.Now().Add(-time.Hour * 24 * 7)) {
			admins, err := m.store.GetMattermostAdminsIds()
			if err != nil {
				m.api.LogDebug("Unable to get the list of Mattermosta admins", "error", err.Error())
				continue
			}
			for _, admin := range admins {
				dm, err := m.api.GetDirectChannel(m.botUserID, admin)
				if err != nil {
					m.api.LogDebug("Error getting direct channel", "error", err.Error())
					continue
				}
				_, err = m.api.CreatePost(&model.Post{
					UserId:    m.botUserID,
					ChannelId: dm.Id,
					Message:   "Your client secret " + credential.Name + " (" + credential.ID + ") will expire in " + credential.ExpireDate.String() + ". Please update it in the settings page.",
				})
				if err != nil {
					m.api.LogDebug("Error sending message", "error", err.Error())
					continue
				}
			}
		}
	}
}
