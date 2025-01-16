// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/api4"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
	"github.com/mattermost/mattermost/server/v8/config"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// mainT is a testing.T-like structure that currently just mimics the t.Cleanup semantics.
type mainT struct {
	cleanupFunctions []func()
}

// Cleanup adds a function to be called when cleaning up.
func (mt *mainT) Cleanup(f func()) {
	mt.cleanupFunctions = append(mt.cleanupFunctions, f)
}

// Done calls all cleanup functions with defer-like semantics (last function added called first).
func (mt *mainT) Done() {
	for i := range mt.cleanupFunctions {
		f := mt.cleanupFunctions[len(mt.cleanupFunctions)-i-1]
		f()
	}
}

func (mt *mainT) Errorf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
	mt.FailNow()
}

func (mt *mainT) FailNow() {
	os.Exit(1)
}

// setupDatabase initializes a singleton Postgres testcontainer and mattermost_test database for
// use with tests.
func setupDatabase(mt *mainT) error {
	// Setup a Postgres testcontainer for all tests.
	pgContainer, err := postgres.Run(context.TODO(), "docker.io/postgres:15.2-alpine",
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

	mt.Cleanup(func() {
		if err := pgContainer.Terminate(context.TODO()); err != nil {
			panic(err)
		}
	})

	return nil
}

var server *app.Server

func getSiteURL() string {
	return fmt.Sprintf("http://localhost:%v", server.ListenAddr.Port)
}

// setupServer initializes a singleton Mattermost instance for use with tests.
func setupServer(mt *mainT) error {
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

	server, err = app.NewServer(options...)
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
	username := model.NewUsername()
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

	return nil
}

var setupReattachEnvironmentOnce sync.Once

// setupReattachEnvironment is used by the test helper to initialize the infrastructure for running
// reattached plugin tests exactly once (per package).
//
// Note that while we assert on the given *testing.T, we setup cleanup functions on the global
// *mainT to clean up once at termination.
func setupReattachEnvironment(mt *mainT) {
	setupReattachEnvironmentOnce.Do(func() {
		err := setupDatabase(mt)
		require.NoError(mt, err)

		err = setupServer(mt)
		require.NoError(mt, err)
	})
}

// TestMain is run before any tests within this package and helps setup a mainT for global cleanup
// if needed.
func TestMain(m *testing.M) {
	var status int
	defer func() {
		os.Exit(status)
	}()

	mt := new(mainT)
	defer mt.Done()

	setupReattachEnvironment(mt)

	// This actually runs the tests
	status = m.Run()
}
