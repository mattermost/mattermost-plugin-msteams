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

	"github.com/mattermost/mattermost-plugin-msteams/server/store/sqlstore"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest/mock"
	mmcontainer "github.com/mattermost/testcontainers-mattermost-go"
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
	cmd.Env = append(cmd.Env, "GO_BUILD_TAGS=msteamsMock")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	require.NoError(t, err)
}

type Option func(*mmcontainer.MattermostContainer)

func WithEnv(key string, value string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env[key] = value
		return nil
	}
}

func WithoutLicense() mmcontainer.MattermostCustomizeRequestOption {
	return func(req *mmcontainer.MattermostContainerRequest) {
		mmcontainer.WithLicense("")(req)
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
		"experimentalsyncchats":      true,
		"synclinkedchannels":         true,
	}

	options := []mmcontainer.MattermostCustomizeRequestOption{
		mmcontainer.WithPlugin(filename, "com.mattermost.msteams-sync", pluginConfig),
		mmcontainer.WithLogConsumers(&tLogConsumer{t: t}),
		mmcontainer.WithNetwork(newNetwork),
		mmcontainer.WithLicense("eyJpZCI6InVjR1kycGNmcjVGSzgwTko5SGVuemhmWDZmIiwiaXNzdWVkX2F0IjoxNzA2OTAyMTE1NTU2LCJzdGFydHNfYXQiOjE3MDY5MDIxMTU1NTYsImV4cGlyZXNfYXQiOjE3NzAwMDg0MDAwMDAsInNrdV9uYW1lIjoiRW50ZXJwcmlzZSIsInNrdV9zaG9ydF9uYW1lIjoiZW50ZXJwcmlzZSIsImN1c3RvbWVyIjp7ImlkIjoicDl1bjM2OWE2N2hpbWo0eWQ2aTZpYjM5YmgiLCJuYW1lIjoiTWF0dGVybW9zdCBFMkUgVGVzdHMiLCJlbWFpbCI6Implc3NlQG1hdHRlcm1vc3QuY29tIiwiY29tcGFueSI6Ik1hdHRlcm1vc3QgRTJFIFRlc3RzIn0sImZlYXR1cmVzIjp7InVzZXJzIjoxMDAsImxkYXAiOnRydWUsImxkYXBfZ3JvdXBzIjp0cnVlLCJtZmEiOnRydWUsImdvb2dsZV9vYXV0aCI6dHJ1ZSwib2ZmaWNlMzY1X29hdXRoIjp0cnVlLCJjb21wbGlhbmNlIjp0cnVlLCJjbHVzdGVyIjp0cnVlLCJtZXRyaWNzIjp0cnVlLCJtaHBucyI6dHJ1ZSwic2FtbCI6dHJ1ZSwiZWxhc3RpY19zZWFyY2giOnRydWUsImFubm91bmNlbWVudCI6dHJ1ZSwidGhlbWVfbWFuYWdlbWVudCI6dHJ1ZSwiZW1haWxfbm90aWZpY2F0aW9uX2NvbnRlbnRzIjp0cnVlLCJkYXRhX3JldGVudGlvbiI6dHJ1ZSwibWVzc2FnZV9leHBvcnQiOnRydWUsImN1c3RvbV9wZXJtaXNzaW9uc19zY2hlbWVzIjp0cnVlLCJjdXN0b21fdGVybXNfb2Zfc2VydmljZSI6dHJ1ZSwiZ3Vlc3RfYWNjb3VudHMiOnRydWUsImd1ZXN0X2FjY291bnRzX3Blcm1pc3Npb25zIjp0cnVlLCJpZF9sb2FkZWQiOnRydWUsImxvY2tfdGVhbW1hdGVfbmFtZV9kaXNwbGF5Ijp0cnVlLCJjbG91ZCI6ZmFsc2UsInNoYXJlZF9jaGFubmVscyI6dHJ1ZSwicmVtb3RlX2NsdXN0ZXJfc2VydmljZSI6dHJ1ZSwib3BlbmlkIjp0cnVlLCJlbnRlcnByaXNlX3BsdWdpbnMiOnRydWUsImFkdmFuY2VkX2xvZ2dpbmciOnRydWUsImZ1dHVyZV9mZWF0dXJlcyI6dHJ1ZX0sImlzX3RyaWFsIjpmYWxzZSwiaXNfZ292X3NrdSI6ZmFsc2V9IMay/e4rVqZ1yEluKxCtWQJK8iWdpADuWyETHJcCDMV8ouQK3n/ocJsg1y7INrbSPZDw6quohjblLN5MqHLQi0c+5yRYwzBhisJD6MFWxFCSg99eSXqIeudAfKVU+WOdZxWhyLzob14hOEfjvN/2hNSNyTV4hqhzk62L9vHzzZsgrFu+zYu4pA6Y4yzZF9FyVvHW241BkGq7ZecmyS6NQsq1jlAhkoBpdW9PCvFDfYS3+CwKtWNfebItc4e9JTbVpo75n++59WV2faQDfiMBf2bYwe6OxzJIZ258r8C2KMFD1uqpQohIoDS9ziygAu2voqgsQqm1Btf1hMtgFAOW7w=="),
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

	api := &plugintest.API{}
	api.On("LogDebug", mock.AnythingOfType("string")).Return()
	api.On("LogDebug", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return()
	api.On("LogDebug", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	api.On("LogInfo", mock.AnythingOfType("string")).Return()
	api.On("LogInfo", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return()
	api.On("LogInfo", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	api.On("LogError", mock.AnythingOfType("string")).Return()
	api.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return()
	api.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	store := sqlstore.New(conn, conn, api, func() []byte { return []byte("eyPBz0mBhwfGGwce9hp4TWaYzgY7MdIB") })
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
