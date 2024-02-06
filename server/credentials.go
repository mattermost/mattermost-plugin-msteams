package main

import (
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
)

func (p *Plugin) checkCredentials() {
	defer func() {
		if r := recover(); r != nil {
			p.GetMetrics().ObserveGoroutineFailure()
			p.API.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	done := p.GetMetrics().ObserveWorker(metrics.WorkerCheckCredentials)
	defer done()

	p.API.LogInfo("Running the check credentials job")

	credentials, err := p.GetClientForApp().GetAppCredentials(p.getConfiguration().ClientID)
	if err != nil {
		p.API.LogWarn("Failed to get app credentials", "error", err.Error())
		return
	}

	// We sort by earliest end date to cover the unlikely event we encounter two credentials
	// with the same hint when reporting the single metric below.
	sort.SliceStable(credentials, func(i, j int) bool {
		return credentials[i].EndDateTime.Before(credentials[j].EndDateTime)
	})

	found := false
	for _, credential := range credentials {
		if strings.HasPrefix(p.getConfiguration().ClientSecret, credential.Hint) {
			p.API.LogInfo("Found matching credential", "credential_name", credential.Name, "credential_id", credential.ID, "credential_end_date_time", credential.EndDateTime)

			if !found {
				// Report the first one that matches the hint.
				p.GetMetrics().ObserveClientSecretEndDateTime(credential.EndDateTime)
			} else {
				// If we happen to get more than one with the same hint, we'll have reported the metric of the
				// earlier one by virtue of the sort above, and we'll have the extra metadata we need in the logs.
				p.API.LogWarn("Found more than one secret with same hint", "credential_id", credential.ID)
			}

			// Note that we keep going to log all the credentials found.
			found = true
		} else {
			p.API.LogInfo("Found other credential", "credential_name", credential.Name, "credential_id", credential.ID, "credential_end_date_time", credential.EndDateTime)
		}
	}

	if !found {
		p.API.LogWarn("Failed to find credential matching configuration")
		p.GetMetrics().ObserveClientSecretEndDateTime(time.Time{})
	}
}
