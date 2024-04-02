package main_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/api4"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
	"github.com/mattermost/mattermost/server/v8/config"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// mainTest is a testing.T-like structure that currently just mimics the t.Cleanup semantics.
type mainTest struct {
	cleanupFunctions []func()
}

// Cleanup adds a function to be called when cleaning up.
func (mt *mainTest) Cleanup(f func()) {
	mt.cleanupFunctions = append(mt.cleanupFunctions, f)
}

// Done calls all cleanup functions with defer-like semantics (last function added called first).
func (mt *mainTest) Done() {
	for i := range mt.cleanupFunctions {
		f := mt.cleanupFunctions[len(mt.cleanupFunctions)-i-1]
		f()
	}
}

// setupServerPath defines MM_SERVER_PATH correctly for server initialization. One day this won't
// be necessary at all.
func setupServerPath() error {
	// Find the server to define MM_SERVER_PATH when running tests.
	serverPath, err := exec.Command("go", "list", "-f", "'{{.Dir}}'", "-m", "github.com/mattermost/mattermost/server/v8").Output()
	if err != nil {
		return err
	}
	os.Setenv("MM_SERVER_PATH", strings.Trim(strings.TrimSpace(string(serverPath)), "'"))

	return nil
}

// setupDatabase initializes a singleton Postgres testcontainer and mattermost_test database for
// use with tests.
func setupDatabase(t *mainTest) error {
	// Setup a Postgres testcontainer for all tests.
	pgContainer, err := postgres.RunContainer(context.TODO(),
		testcontainers.WithImage("docker.io/postgres:15.2-alpine"),
		postgres.WithDatabase("mattermost_test"),
		postgres.WithUsername("mmuser"),
		postgres.WithPassword("mostest"),
		// network.WithNetwork([]string{"db"}, nw),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		return err
	}

	containerPort, err := pgContainer.MappedPort(context.TODO(), "5432/tcp")
	if err != nil {
		return err
	}

	postgresDSN := fmt.Sprintf("postgres://mmuser:mostest@%s/mattermost_test?sslmode=disable", net.JoinHostPort("localhost", containerPort.Port()))
	os.Setenv("TEST_DATABASE_POSTGRESQL_DSN", postgresDSN)

	t.Cleanup(func() {
		if err := pgContainer.Terminate(context.TODO()); err != nil {
			panic(err)
		}
	})

	return nil
}

// setupServer initializes a singleton Mattermost instance for use with tests.
func setupServer(mt *mainTest) error {
	// Ignore any locally defined SiteURL as we intend to host our own.
	os.Unsetenv("MM_SERVICESETTINGS_SITEURL")
	os.Unsetenv("MM_SERVICESETTINGS_LISTENADDRESS")

	tmpDir, err := os.MkdirTemp("", "msteams")
	if err != nil {
		return err
	}
	mt.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	// Execute from the temporary directory to avoid polluting the developer's working
	// directory and simplify cleanup.
	err = os.Chdir(tmpDir)
	if err != nil {
		return err
	}

	// Setup a custom MM_LOCALSOCKETPATH.
	os.Setenv("MM_LOCALSOCKETPATH", path.Join(tmpDir, "mattermost_local.socket"))

	// Create a test memory store and modify configuration appropriately
	configStore := config.NewTestMemoryStore()
	config := configStore.Get()
	config.PluginSettings.Directory = model.NewString(path.Join(tmpDir, "plugins"))
	config.PluginSettings.ClientDirectory = model.NewString(path.Join(tmpDir, "client"))
	config.ServiceSettings.ListenAddress = model.NewString("localhost:0")
	config.TeamSettings.MaxUsersPerTeam = model.NewInt(10000)
	config.LocalizationSettings.SetDefaults()
	config.SqlSettings = *storetest.MakeSqlSettings("postgres", false)
	config.ServiceSettings.SiteURL = model.NewString("http://example.com/")
	config.LogSettings.EnableConsole = model.NewBool(true)
	config.LogSettings.EnableFile = model.NewBool(false)
	config.LogSettings.ConsoleLevel = model.NewString("DEBUG")
	config.ServiceSettings.EnableLocalMode = model.NewBool(true)
	config.ServiceSettings.LocalModeSocketLocation = model.NewString(path.Join(tmpDir, "mattermost_local.socket"))
	config.ServiceSettings.EnableDeveloper = model.NewBool(true)
	config.ServiceSettings.EnableTesting = model.NewBool(true)
	config.FileSettings.Directory = model.NewString(path.Join(tmpDir, "data"))

	_, _, err = configStore.Set(config)
	if err != nil {
		return err
	}

	// Create a logger to override
	testLogger, err := mlog.NewLogger()
	if err != nil {
		return err
	}
	testLogger.LockConfiguration()

	// Initialize the server with app and api4 interfaces.
	options := []app.Option{
		app.ConfigStore(configStore),
	}

	server, err := app.NewServer(options...)
	if err != nil {
		return err
	}

	_, err = api4.Init(server)
	if err != nil {
		return err
	}

	err = server.Start()
	if err != nil {
		return err
	}
	mt.Cleanup(func() {
		server.Shutdown()
	})

	ap := app.New(app.ServerConnector(server.Channels()))

	// Setup the first user immediately.
	username := model.NewId()
	user := &model.User{
		Email:         fmt.Sprintf("%s@example.com", username),
		Username:      username,
		Password:      "password",
		EmailVerified: true,
	}

	_, appErr := ap.CreateUser(request.EmptyContext(testLogger), user)
	if appErr != nil {
		return appErr
	}

	// Export the site url for later use in tests.
	os.Setenv("MM_SERVICESETTINGS_SITEURL", fmt.Sprintf("http://localhost:%v", server.ListenAddr.Port))

	return nil
}

// TestMain is run before any tests within this package and helps setup a singleton Postgres and
// Mattermost intance for use with tests.
func TestMain(m *testing.M) {
	mt := new(mainTest)
	defer mt.Done()

	err := setupServerPath()
	if err != nil {
		panic("failed to setup server path: " + err.Error())
	}

	err = setupDatabase(mt)
	if err != nil {
		panic("failed to setup database: " + err.Error())
	}

	err = setupServer(mt)
	if err != nil {
		panic("failed to setup server: " + err.Error())
	}

	// This actually runs the tests
	status := m.Run()

	os.Exit(status)
}
