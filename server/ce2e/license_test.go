package ce2e

import (
	"context"
	"strings"
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams/server/testutils/containere2e"
	"github.com/stretchr/testify/require"
)

func TestRequiresLicense(t *testing.T) {
	t.Parallel()
	mattermost, _, tearDown := containere2e.NewE2ETestPlugin(t, containere2e.WithoutLicense())
	defer tearDown()

	client, err := mattermost.GetAdminClient(context.Background())
	require.NoError(t, err)

	t.Run("without license", func(t *testing.T) {
		plugins, _, err := client.GetPlugins(context.Background())
		require.NoError(t, err)

		for _, plugin := range plugins.Active {
			require.NotEqual(t, "com.mattermost.msteams-sync", plugin.Manifest.Id)
		}

		found := false
		for _, plugin := range plugins.Inactive {
			if plugin.Manifest.Id == "com.mattermost.msteams-sync" {
				found = true
				break
			}
		}
		if !found {
			require.Fail(t, "failed to find plugin status at all")
		}

		rawLogs, err := mattermost.GetLogs(context.Background(), 1000)
		require.NoError(t, err)

		found = false
		logs := strings.Split(rawLogs, "\n")
		for _, log := range logs {
			if strings.Contains(log, "this plugin requires an enterprise license") {
				found = true
				break
			}
		}
		if !found {
			require.Fail(t, "failed to find expected error log")
		}
	})
}
