package main

import (
	"runtime/debug"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
)

func (p *Plugin) checkCredentials() {
	defer func() {
		if r := recover(); r != nil {
			p.GetMetrics().ObserveGoroutineFailure()
			p.API.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	applicationID := p.getConfiguration().ApplicationID

	if applicationID == "" {
		p.API.LogDebug("Skipping credentials check since no application id configured.")
		return
	}

	done := p.GetMetrics().ObserveWorker(metrics.WorkerCheckCredentials)
	defer done()

	credentials, err := p.GetClientForApp().GetAppCredentials(applicationID)
	if err != nil {
		p.API.LogWarn("Failed to get app credentials", "error", err.Error())
		return
	}

	for _, credential := range credentials {
		p.GetMetrics().ObserveClientSecretEndDateTime(credential.ID, credential.EndDateTime)
	}
}
