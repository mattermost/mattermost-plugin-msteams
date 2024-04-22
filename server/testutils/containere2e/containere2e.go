package containere2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mockserver"
	"github.com/testcontainers/testcontainers-go/network"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/sqlstore"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils/mmcontainer"
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
	cmd.Env = append(cmd.Env, "GO_BUILD_TAGS=msteamsMock")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	require.NoError(t, err)
}

type Option func(*mmcontainer.MattermostContainer)

func WithEnv(key string, value string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
		req.Env[key] = value
	}
}

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
		_ = newNetwork.Remove(context.Background())
		t.Fatal(err)
	}
	if len(matches) == 0 {
		_ = newNetwork.Remove(context.Background())
		t.Fatal("Unable to find plugin tar.gz file")
	}
	filename := matches[0]

	mockserverContainer, err := mockserver.RunContainer(
		context.Background(),
		network.WithNetwork([]string{"mockserver"}, newNetwork),
		WithEnv("MOCKSERVER_ATTEMPT_TO_PROXY_IF_NO_MATCHING_EXPECTATION", "false"),
	)
	if err != nil {
		_ = newNetwork.Remove(context.Background())
		t.Fatal(err)
	}

	mockAPIURL, err := mockserverContainer.URL(context.Background())
	if err != nil {
		_ = mockserverContainer.Terminate(context.Background())
		_ = newNetwork.Remove(context.Background())
		t.Fatal(err)
	}

	if os.Getenv("INSPECT_MOCKSERVER") != "" {
		t.Logf("Mockserver URL: %s\n", mockAPIURL+"/mockserver/dashboard")
	}

	mockClient, err := NewMockClient(mockAPIURL)
	if err != nil {
		_ = mockserverContainer.Terminate(context.Background())
		_ = newNetwork.Remove(context.Background())
		t.Fatal(err)
	}

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
		"syncreactions":              true,
		"disableSyncMsg":             false,
		"useSharedChannels":          true,
	}

	options := []mmcontainer.MattermostCustomizeRequestOption{
		mmcontainer.WithPlugin(filename, "com.mattermost.msteams-sync", pluginConfig),
		mmcontainer.WithLogConsumers(&tLogConsumer{t: t}),
		mmcontainer.WithEnv("MM_EXPERIMENTALSETTINGS_ENABLESHAREDCHANNELS", "true"),
		mmcontainer.WithEnv("MM_LOGSETTINGS_ENABLECONSOLE", "false"),
		mmcontainer.WithEnv("MM_LOGSETTINGS_ADVANCEDLOGGINGJSON", getLoggingConfig()),
		mmcontainer.WithNetwork(newNetwork),
	}
	options = append(options, extraOptions...)
	mattermost, err := mmcontainer.RunContainer(ctx, options...)

	require.NoError(t, err)
	if err != nil {
		_ = mockserverContainer.Terminate(context.Background())
		_ = mattermost.Terminate(ctx)
		_ = newNetwork.Remove(context.Background())
		t.Fatal(err)
	}

	conn, err := mattermost.PostgresConnection(ctx)
	if err != nil {
		_ = mockserverContainer.Terminate(ctx)
		_ = mattermost.Terminate(ctx)
		_ = newNetwork.Remove(context.Background())
	}
	require.NoError(t, err)

	store := sqlstore.New(conn, nil, func() []string { return []string{""} }, func() []byte { return []byte("eyPBz0mBhwfGGwce9hp4TWaYzgY7MdIB") })
	if err2 := store.Init(""); err2 != nil {
		_ = mockserverContainer.Terminate(ctx)
		_ = mattermost.Terminate(ctx)
		_ = newNetwork.Remove(context.Background())
	}
	require.NoError(t, err)

	if os.Getenv("INSPECT_MOCKSERVER") != "" {
		t.Logf("Mockserver URL: %s\n", mockAPIURL+"/mockserver/dashboard")
	}

	tearDown := func() {
		if os.Getenv("INSPECT_MOCKSERVER") != "" {
			t.Logf("Mockserver URL: %s\n", mockAPIURL+"/mockserver/dashboard")
			t.Logf("Press the Enter Key to stop the mock server and continue to the test results!")
			fmt.Scanln()
		}

		require.NoError(t, mockserverContainer.Terminate(context.Background()))
		require.NoError(t, mattermost.Terminate(context.Background()))
		require.NoError(t, newNetwork.Remove(context.Background()))
	}

	return mattermost, store, mockClient, tearDown
}

func getLoggingConfig() string {
	return `{
		"test-console":{
			"Type": "console",
			"Format": "plain",
			"Levels": [
					{"ID": 5, "Name": "debug", "Stacktrace": false},
					{"ID": 4, "Name": "info", "Stacktrace": false},
					{"ID": 3, "Name": "warn", "Stacktrace": false},
					{"ID": 2, "Name": "error", "Stacktrace": false},
					{"ID": 1, "Name": "fatal", "Stacktrace": true},
					{"ID": 0, "Name": "panic", "Stacktrace": true},
					{"ID": 130, "Name": "RemoteClusterServiceDebug", "Stacktrace": false},
					{"ID": 131, "Name": "RemoteClusterServiceError", "Stacktrace": true},
					{"ID": 200, "Name": "SharedChannelServiceDebug", "Stacktrace": false},
					{"ID": 201, "Name": "SharedChannelServiceError", "Stacktrace": true}
			],
			"Options": {
				"Out": "stdout"
			},
			"MaxQueueSize": 1000
		}
	}`
}
