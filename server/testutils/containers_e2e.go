package testutils

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/sqlstore"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils/mmcontainer"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils/testmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func NewE2ETestPlugin(t *testing.T) (*mmcontainer.MattermostContainer, *sqlstore.SQLStore, func(ctx context.Context) error) {
	ctx := context.Background()
	matches, err := filepath.Glob("../dist/*.tar.gz")
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
		mmcontainer.WithPlugin("../dist/"+filename, "com.mattermost.msteams-sync", pluginConfig),
		mmcontainer.WithEnv("MM_MSTEAMSSYNC_MOCK_CLIENT", "true"),
	)

	conn, err := mattermost.PostgresConnection(ctx)
	if err != nil {
		mattermost.Terminate(ctx)
	}
	require.NoError(t, err)

	store := sqlstore.New(conn, "postgres", nil, func() []string { return []string{""} }, func() []byte { return []byte("eyPBz0mBhwfGGwce9hp4TWaYzgY7MdIB") })
	store.Init()

	return mattermost, store, mattermost.Terminate
}

func MockMSTeamsClient(t *testing.T, client *model.Client4, method string, returnType string, returns interface{}, returnErr string) {
	mockStruct := testmodels.MockCallReturns{ReturnType: returnType, Returns: returns, Err: returnErr}
	mockData, err := json.Marshal(mockStruct)
	require.NoError(t, err)

	_, err = client.DoAPIRequest(context.Background(), http.MethodPost, client.URL+"/plugins/com.mattermost.msteams-sync/add-mock/"+method, string(mockData), "")
	require.NoError(t, err)
}
