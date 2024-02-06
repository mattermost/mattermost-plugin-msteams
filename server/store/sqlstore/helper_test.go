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
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var db *sql.DB

func TestMain(m *testing.M) {
	var tearDown func()
	db, tearDown = createTestDB()
	retCode := m.Run()
	tearDown()

	os.Exit(retCode)
}

func createTestDB() (*sql.DB, func()) {
	context := context.Background()
	postgres, _ := testcontainers.GenericContainer(context,
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

	driverName := model.DatabaseDriverPostgres
	host, _ := postgres.Host(context)
	hostPort, _ := postgres.MappedPort(context, "5432/tcp")
	conn, err := sqlx.Connect(driverName, fmt.Sprintf("%s://user:pass@%s:%d?sslmode=disable", driverName, host, hostPort.Int()))
	if err != nil {
		panic("failed to connect to test db: " + err.Error())
	}
	tearDownContainer := func() {
		if err := postgres.Terminate(context); err != nil {
			log.Fatalf("failed to terminate container: %s", err.Error())
		}
	}

	return conn.DB, tearDownContainer
}

func setupTestStore(t *testing.T) (*SQLStore, *plugintest.API) {
	api := &plugintest.API{}

	store := &SQLStore{}
	store.api = api
	store.db = db

	_ = store.Init()
	_ = store.createTable("Teams", "Id VARCHAR(255), DisplayName VARCHAR(255)")
	_ = store.createTable("Channels", "Id VARCHAR(255), DisplayName VARCHAR(255)")
	_ = store.createTable("Users", "Id VARCHAR(255), FirstName VARCHAR(255), LastName VARCHAR(255), Email VARCHAR(255)")

	return store, api
}
