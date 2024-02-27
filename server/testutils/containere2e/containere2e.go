package containere2e

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/sqlstore"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils/mmcontainer"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils/testmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mockserver"
	"github.com/testcontainers/testcontainers-go/network"
)

type tLogConsumer struct {
	t *testing.T
}

func (tlc *tLogConsumer) Accept(log testcontainers.Log) {
	tlc.t.Log(strings.TrimSpace(string(log.Content)))
}

var buildPluginOnce sync.Once

func buildPlugin(t *testing.T) {
	cmd := exec.Command("make", "-C", "../../", "dist")
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DEFAULT_GOOS=linux")
	cmd.Env = append(cmd.Env, "DEFAULT_GOARCH=amd64")
	cmd.Env = append(cmd.Env, "MM_SERVICESETTINGS_ENABLEDEVELOPER=true")
	cmd.Env = append(cmd.Env, "GO_BUILD_TAGS=msteamsMock")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	require.NoError(t, err)
}

type Option func(*mmcontainer.MattermostContainer)

func WithoutLicense() mmcontainer.MattermostCustomizeRequestOption {
	return func(req *mmcontainer.MattermostContainerRequest) {
		mmcontainer.WithEnv("MM_LICENSE", "")(req)
	}
}

func NewE2ETestPlugin(t *testing.T, extraOptions ...mmcontainer.MattermostCustomizeRequestOption) (*mmcontainer.MattermostContainer, *sqlstore.SQLStore, *MockClient, func()) {
	buildPluginOnce.Do(func() {
		buildPlugin(t)
	})

	newNetwork, err := network.New(context.Background(), network.WithCheckDuplicate())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	matches, err := filepath.Glob("../../dist/*.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("Unable to find plugin tar.gz file")
	}
	filename := matches[0]

	mockserverContainer, err := mockserver.RunContainer(context.Background(), network.WithNetwork([]string{"mockserver"}, newNetwork))
	mockAPIURL, err := mockserverContainer.URL(context.Background())
	if err != nil {
		_ = mockserverContainer.Terminate(context.Background())
		t.Fatal(err)
	}
	mockClient, err := NewMockClient(mockAPIURL)
	if err != nil {
		_ = mockserverContainer.Terminate(context.Background())
		t.Fatal(err)
	}
	t.Log("MOCKSERVER DASHBOARD:", mockAPIURL+"/mockserver/dashboard")

	pluginConfig := map[string]any{
		"clientid":                   "client-id",
		"clientsecret":               "client-secret",
		"connectedusersallowed":      1000,
		"encryptionkey":              "eyPBz0mBhwfGGwce9hp4TWaYzgY7MdIB",
		"maxSizeForCompleteDownload": 20,
		"maxsizeforcompletedownload": 20,
		"tenantid":                   "tenant-id",
		"webhooksecret":              "webhook-secret",
		"syncdirectmessages":         true,
		"synclinkedchannels":         true,
	}

	options := []mmcontainer.MattermostCustomizeRequestOption{
		mmcontainer.WithPlugin(filename, "com.mattermost.msteams-sync", pluginConfig),
		mmcontainer.WithLogConsumers(&tLogConsumer{t: t}),
		mmcontainer.WithNetwork(newNetwork),
	}
	options = append(options, extraOptions...)
	mattermost, err := mmcontainer.RunContainer(ctx, options...)

	require.NoError(t, err)
	if err != nil {
		_ = mockserverContainer.Terminate(context.Background())
		_ = mattermost.Terminate(ctx)
		t.Fatal(err)
	}

	conn, err := mattermost.PostgresConnection(ctx)
	if err != nil {
		_ = mockserverContainer.Terminate(ctx)
		_ = mattermost.Terminate(ctx)
	}
	require.NoError(t, err)

	store := sqlstore.New(conn, nil, func() []string { return []string{""} }, func() []byte { return []byte("eyPBz0mBhwfGGwce9hp4TWaYzgY7MdIB") })
	if err2 := store.Init(); err2 != nil {
		_ = mockserverContainer.Terminate(ctx)
		_ = mattermost.Terminate(ctx)
	}
	require.NoError(t, err)

	tearDown := func() {
		require.NoError(t, mockserverContainer.Terminate(context.Background()))
		require.NoError(t, mattermost.Terminate(context.Background()))
	}

	return mattermost, store, mockClient, tearDown
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
