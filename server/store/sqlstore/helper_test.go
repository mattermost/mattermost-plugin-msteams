package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var db *sql.DB

func TestMain(m *testing.M) {
	var tearDown func()
	var err error
	db, tearDown, err = createTestDB()
	if err != nil {
		panic("failed to create test db: " + err.Error())
	}
	retCode := m.Run()
	tearDown()

	os.Exit(retCode)
}

func createTestDB() (*sql.DB, func(), error) {
	context := context.Background()
	postgres, err := testcontainers.GenericContainer(context,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "postgres",
				ExposedPorts: []string{"5432/tcp"},
				Env: map[string]string{
					"POSTGRES_PASSWORD": "pass",
					"POSTGRES_USER":     "user",
				},
				WaitingFor: wait.ForAll(
					wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
				),
				SkipReaper: true,
			},
			Started: true,
		})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create postgres container")
	}

	driverName := model.DatabaseDriverPostgres
	host, err := postgres.Host(context)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get postgres host")
	}
	hostPort, err := postgres.MappedPort(context, "5432/tcp")
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to map postgres port")
	}
	conn, err := sqlx.Connect(driverName, fmt.Sprintf("%s://user:pass@%s:%d?sslmode=disable", driverName, host, hostPort.Int()))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect to test db")
	}
	tearDownContainer := func() {
		if err := postgres.Terminate(context); err != nil {
			log.Fatalf("failed to terminate container: %s", err.Error())
		}
	}

	return conn.DB, tearDownContainer, nil
}

func setupTestStore(t *testing.T) (*SQLStore, *plugintest.API) {
	api := &plugintest.API{}
	// mock logger calls
	api.On("LogInfo", mock.AnythingOfType("string")).Return()
	api.On("LogInfo", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return()
	api.On("LogDebug", mock.AnythingOfType("string")).Return()
	api.On("LogDebug", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return()
	api.On("LogError", mock.AnythingOfType("string")).Return()
	api.On("LogError", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return()

	store := &SQLStore{}
	store.api = api
	store.db = db

	err := store.createTable("Teams", "Id VARCHAR(255), DisplayName VARCHAR(255)")
	require.NoError(t, err)
	err = store.createTable("Channels", "Id VARCHAR(255), DisplayName VARCHAR(255)")
	require.NoError(t, err)
	err = store.createTable("Users", "Id VARCHAR(255), FirstName VARCHAR(255), LastName VARCHAR(255), Email VARCHAR(255), remoteid VARCHAR(26), createat BIGINT")
	require.NoError(t, err)
	err = store.Init("")
	require.NoError(t, err)

	return store, api
}
