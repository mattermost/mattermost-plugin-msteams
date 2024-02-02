package ce2e

import (
	"context"
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils/containere2e"
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

		for _, plugin := range plugins.Inactive {
			if plugin.Manifest.Id == "com.mattermost.msteams-sync" {
				return
			}
		}
		require.Fail(t, "failed to find plugin status at all")
	})
}
