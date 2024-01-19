package containere2e

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/sqlstore"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils/mmcontainer"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils/testmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func NewE2ETestPlugin(t *testing.T) (*mmcontainer.MattermostContainer, *sqlstore.SQLStore, func()) {
	cmd := exec.Command("make", "-C", "../../", "dist")
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DEFAULT_GOOS=linux")
	cmd.Env = append(cmd.Env, "DEFAULT_GOARCH=amd64")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	require.NoError(t, err)

	ctx := context.Background()
	matches, err := filepath.Glob("../../dist/*.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("Unable to find plugin tar.gz file")
	}
	filename := matches[0]

	pluginConfig := map[string]any{
		"clientid":                   "client-id",
		"clientsecret":               "client-secret",
		"connectedusersallowed":      1000,
		"encryptionkey":              "eyPBz0mBhwfGGwce9hp4TWaYzgY7MdIB",
		"maxSizeForCompleteDownload": 20,
		"maxsizeforcompletedownload": 20,
		"tenantid":                   "tenant-id",
		"webhooksecret":              "webhook-secret",
	}

	mattermost, err := mmcontainer.RunContainer(ctx,
		mmcontainer.WithPlugin(filename, "com.mattermost.msteams-sync", pluginConfig),
		mmcontainer.WithEnv("MM_MSTEAMSSYNC_MOCK_CLIENT", "true"),
		mmcontainer.WithTestingLogConsumer(t),
	)
	require.NoError(t, err)

	// TODO: This won't be required after jespino gets https://github.com/testcontainers/testcontainers-go/pull/2073 merged.
	err = mattermost.StartLogProducer(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = mattermost.StopLogProducer()
		require.NoError(t, err)
	})

	conn, err := mattermost.PostgresConnection(ctx)
	if err != nil {
		_ = mattermost.Terminate(ctx)
	}
	require.NoError(t, err)

	store := sqlstore.New(conn, "postgres", nil, func() []string { return []string{""} }, func() []byte { return []byte("eyPBz0mBhwfGGwce9hp4TWaYzgY7MdIB") })
	if err2 := store.Init(); err2 != nil {
		_ = mattermost.Terminate(ctx)
	}
	require.NoError(t, err)

	tearDown := func() {
		require.NoError(t, mattermost.Terminate(context.Background()))
	}

	return mattermost, store, tearDown
}

func MockMSTeamsClient(t *testing.T, client *model.Client4, method string, returnType string, returns interface{}, returnErr string) {
	mockStruct := testmodels.MockCallReturns{ReturnType: returnType, Returns: returns, Err: returnErr}
	mockData, err := json.Marshal(mockStruct)
	require.NoError(t, err)

	resp, err := client.DoAPIRequest(context.Background(), http.MethodPost, client.URL+"/plugins/com.mattermost.msteams-sync/add-mock/"+method, string(mockData), "")
	require.NoError(t, err)
	resp.Body.Close()
}

func ResetMSTeamsClientMock(t *testing.T, client *model.Client4) {
	resp, err := client.DoAPIRequest(context.Background(), http.MethodPost, client.URL+"/plugins/com.mattermost.msteams-sync/reset-mocks", "", "")
	require.NoError(t, err)
	resp.Body.Close()
}
