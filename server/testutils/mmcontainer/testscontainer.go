package mmcontainer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultEmail           = "admin@example.com"
	defaultUsername        = "admin"
	defaultPassword        = "admin"
	defaultTeamName        = "test"
	defaultTeamDisplayName = "Test"
	defaultMattermostImage = "mattermost/mattermost-enterprise-edition"
)

type MattermostCustomizeRequestOption func(req *MattermostContainerRequest)

type plugin struct {
	path   string
	id     string
	config map[string]any
}

type MattermostContainerRequest struct {
	testcontainers.GenericContainerRequest
	email           string
	username        string
	password        string
	teamName        string
	teamDisplayName string
	plugins         []plugin
}

// MattermostContainer represents the mattermost container type used in the module
type MattermostContainer struct {
	testcontainers.Container
	pgContainer *postgres.PostgresContainer
	network     *testcontainers.DockerNetwork
	username    string
	password    string
}

func (c *MattermostContainer) URL(ctx context.Context) (string, error) {
	containerPort, err := c.MappedPort(ctx, "8065/tcp")
	if err != nil {
		return "", err
	}

	host, err := c.Host(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("http://%s", net.JoinHostPort(host, containerPort.Port())), nil
}

func (c *MattermostContainer) GetAdminClient(ctx context.Context) (*model.Client4, error) {
	url, err := c.URL(ctx)
	if err != nil {
		return nil, err
	}
	client := model.NewAPIv4Client(url)
	_, _, err = client.Login(context.Background(), c.username, c.password)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (c *MattermostContainer) PostgresConnection(ctx context.Context) (*sql.DB, error) {
	postgresDSN, err := c.PostgresDSN(ctx)
	if err != nil {
		return nil, err
	}

	conn, err := sql.Open("postgres", postgresDSN)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *MattermostContainer) PostgresDSN(ctx context.Context) (string, error) {
	containerPort, err := c.pgContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return "", err
	}

	host, err := c.Host(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("postgres://user:pass@%s/mattermost_test?sslmode=disable", net.JoinHostPort(host, containerPort.Port())), nil
}

func (c *MattermostContainer) Terminate(ctx context.Context) error {
	var errors error
	if err := c.pgContainer.Terminate(ctx); err != nil {
		errors = fmt.Errorf("%w + %w", errors, err)
	}

	if err := c.Container.Terminate(ctx); err != nil {
		errors = fmt.Errorf("%w + %w", errors, err)
	}

	if err := c.network.Remove(ctx); err != nil {
		errors = fmt.Errorf("%w + %w", errors, err)
	}

	return errors
}

func (c *MattermostContainer) CreateAdmin(ctx context.Context, email, username, password string) error {
	_, _, err := c.Exec(ctx, []string{"mmctl", "--local", "user", "create", "--email", email, "--username", username, "--password", password, "--system-admin", "--email-verified"})
	return err
}

func (c *MattermostContainer) CreateUser(ctx context.Context, email, username, password string) error {
	_, _, err := c.Exec(ctx, []string{"mmctl", "--local", "user", "create", "--email", email, "--username", username, "--password", password, "--email-verified"})
	return err
}

func (c *MattermostContainer) CreateTeam(ctx context.Context, name, displayName string) error {
	_, _, err := c.Exec(ctx, []string{"mmctl", "--local", "team", "create", "--name", name, "--display-name", displayName})
	return err
}

func (c *MattermostContainer) AddUserToTeam(ctx context.Context, username, teamname string) error {
	_, _, err := c.Exec(ctx, []string{"mmctl", "--local", "team", "users", "add", teamname, username})
	return err
}

func (c *MattermostContainer) GetLogs(ctx context.Context, lines int) (string, error) {
	_, output, err := c.Exec(ctx, []string{"mmctl", "--local", "logs", "--number", fmt.Sprintf("%d", lines)})
	if err != nil {
		return "", err
	}
	outputData, err := io.ReadAll(output)
	if err != nil {
		return "", err
	}
	return string(outputData), nil
}

func (c *MattermostContainer) setSiteURL(ctx context.Context) error {
	url, err := c.URL(ctx)
	if err != nil {
		return err
	}
	_, _, err = c.Exec(ctx, []string{"mmctl", "--local", "config", "set", "ServiceSettings.SiteURL", url})
	if err != nil {
		return err
	}
	containerPort, err := c.MappedPort(ctx, "8065/tcp")
	if err != nil {
		return err
	}
	_, _, err = c.Exec(ctx, []string{"mmctl", "--local", "config", "set", "ServiceSettings.ListenAddress", containerPort.Port()})
	return err
}

func (c *MattermostContainer) InstallPlugin(ctx context.Context, pluginPath string, pluginID string, pluginConfig map[string]any) error {
	patch := map[string]map[string]map[string]map[string]any{"PluginSettings": {"Plugins": {pluginID: pluginConfig}}}
	config, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())

	_, err = f.Write([]byte(config))
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}

	_, _, err = c.Exec(ctx, []string{"mmctl", "--local", "plugin", "add", pluginPath})
	if err != nil {
		return err
	}

	configPath := "/tmp/plugin-config-" + pluginID + ".json"
	err = c.CopyFileToContainer(ctx, f.Name(), configPath, 0o755)
	if err != nil {
		return err
	}
	defer func() {
		_, _, _ = c.Exec(ctx, []string{"rm", "-f", configPath})
	}()

	_, _, err = c.Exec(ctx, []string{"mmctl", "--local", "config", "patch", configPath})
	if err != nil {
		return err
	}

	_, _, err = c.Exec(ctx, []string{"mmctl", "--local", "plugin", "enable", pluginID})
	return err
}

// WithConfigFile sets the config file to be used for the postgres container
// It will also set the "config_file" parameter to the path of the config file
// as a command line argument to the container
func WithConfigFile(cfg string) MattermostCustomizeRequestOption {
	return func(req *MattermostContainerRequest) {
		cfgFile := testcontainers.ContainerFile{
			HostFilePath:      cfg,
			ContainerFilePath: "/etc/mattermost.json",
			FileMode:          0o755,
		}

		req.Files = append(req.Files, cfgFile)
		req.Cmd = append(req.Cmd, "-c", "/etc/mattermost.json")
	}
}

// WithInitScripts sets the init scripts to be run when the container starts
func WithInitScripts(scripts ...string) MattermostCustomizeRequestOption {
	return func(req *MattermostContainerRequest) {
		initScripts := []testcontainers.ContainerFile{}
		for _, script := range scripts {
			cf := testcontainers.ContainerFile{
				HostFilePath:      script,
				ContainerFilePath: "/docker-entrypoint-initdb.d/" + filepath.Base(script),
				FileMode:          0o755,
			}
			initScripts = append(initScripts, cf)
		}
		req.Files = append(req.Files, initScripts...)
	}
}

// WithEnv sets the environment variable to the given value
func WithEnv(env, value string) MattermostCustomizeRequestOption {
	return func(req *MattermostContainerRequest) {
		req.Env[env] = value
	}
}

// WithAdmin sets the admin email, username and password for the mattermost container
func WithAdmin(email, username, password string) MattermostCustomizeRequestOption {
	return func(req *MattermostContainerRequest) {
		req.email = email
		req.username = username
		req.password = password
	}
}

// WithTeam sets the team name and display name for the mattermost container
func WithTeam(teamName, teamDisplayName string) MattermostCustomizeRequestOption {
	return func(req *MattermostContainerRequest) {
		req.teamName = teamName
		req.teamDisplayName = teamDisplayName
	}
}

// WithPlugin sets the plugin to be installed in the mattermost container
func WithPlugin(pluginPath, pluginID string, pluginConfig map[string]any) MattermostCustomizeRequestOption {
	return func(req *MattermostContainerRequest) {
		uuid, _ := uuid.NewUUID()

		pluginFile := testcontainers.ContainerFile{
			HostFilePath:      pluginPath,
			ContainerFilePath: fmt.Sprintf("/tmp/%s.tar.gz", uuid.String()),
			FileMode:          0o755,
		}

		req.Files = append(req.Files, pluginFile)
		req.plugins = append(req.plugins, plugin{
			path:   fmt.Sprintf("/tmp/%s.tar.gz", uuid.String()),
			id:     pluginID,
			config: pluginConfig,
		})
	}
}

func runPostgresContainer(ctx context.Context, nw *testcontainers.DockerNetwork) (*postgres.PostgresContainer, error) {
	return postgres.RunContainer(ctx,
		testcontainers.WithImage("docker.io/postgres:15.2-alpine"),
		postgres.WithDatabase("mattermost_test"),
		postgres.WithUsername("user"),
		postgres.WithPassword("pass"),
		network.WithNetwork([]string{"db"}, nw),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
}

// RunContainer creates an instance of the postgres container type
func RunContainer(ctx context.Context, opts ...MattermostCustomizeRequestOption) (*MattermostContainer, error) {
	newNetwork, err := network.New(ctx, network.WithCheckDuplicate())
	if err != nil {
		return nil, err
	}

	postgresContainer, err := runPostgresContainer(ctx, newNetwork)
	if err != nil {
		if err2 := newNetwork.Remove(ctx); err2 != nil {
			err = fmt.Errorf("%w + %w", err, err2)
		}
		return nil, err
	}

	dbconn := fmt.Sprintf("postgres://user:pass@%s:%d/mattermost_test?sslmode=disable", "db", 5432)
	req := MattermostContainerRequest{
		GenericContainerRequest: testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image: defaultMattermostImage,
				Env: map[string]string{
					"MM_SQLSETTINGS_DATASOURCE":          dbconn,
					"MM_SQLSETTINGS_DRIVERNAME":          "postgres",
					"MM_SERVICESETTINGS_ENABLELOCALMODE": "true",
					"MM_PASSWORDSETTINGS_MINIMUMLENGTH":  "5",
					"MM_PLUGINSETTINGS_ENABLEUPLOADS":    "true",
					"MM_FILESETTINGS_MAXFILESIZE":        "256000000",
					"MM_LOGSETTINGS_CONSOLELEVEL":        "DEBUG",
					"MM_LOGSETTINGS_FILELEVEL":           "DEBUG",
				},
				ExposedPorts: []string{"8065/tcp"},
				Cmd:          []string{"mattermost", "server"},
				WaitingFor: wait.ForAll(
					wait.ForLog("Server is listening on"),
				).WithDeadline(30 * time.Second),
				Networks:       []string{newNetwork.Name},
				NetworkAliases: map[string][]string{newNetwork.Name: {"mattermost"}},
			},
			Started: true,
		},
		email:           defaultEmail,
		username:        defaultUsername,
		password:        defaultPassword,
		teamName:        defaultTeamName,
		teamDisplayName: defaultTeamDisplayName,
	}

	for _, opt := range opts {
		opt(&req)
	}

	container, err := testcontainers.GenericContainer(ctx, req.GenericContainerRequest)
	if err != nil {
		if err2 := postgresContainer.Terminate(ctx); err2 != nil {
			err = fmt.Errorf("%w + %w", err, err2)
		}
		if err2 := newNetwork.Remove(ctx); err2 != nil {
			err = fmt.Errorf("%w + %w", err, err2)
		}
		return nil, err
	}

	mattermost := &MattermostContainer{
		Container:   container,
		pgContainer: postgresContainer,
		network:     newNetwork,
		username:    req.username,
		password:    req.password,
	}

	if err := mattermost.setSiteURL(context.Background()); err != nil {
		if err2 := mattermost.Terminate(ctx); err2 != nil {
			err = fmt.Errorf("%w + %w", err, err2)
		}
		return nil, err
	}

	if err := mattermost.CreateAdmin(ctx, req.email, req.username, req.password); err != nil {
		if err2 := mattermost.Terminate(ctx); err2 != nil {
			err = fmt.Errorf("%w + %w", err, err2)
		}
		return nil, err
	}

	if err := mattermost.CreateTeam(ctx, req.teamName, req.teamDisplayName); err != nil {
		if err2 := mattermost.Terminate(ctx); err2 != nil {
			err = fmt.Errorf("%w + %w", err, err2)
		}
		return nil, err
	}

	if err := mattermost.AddUserToTeam(ctx, req.username, req.teamName); err != nil {
		if err2 := mattermost.Terminate(ctx); err2 != nil {
			err = fmt.Errorf("%w + %w", err, err2)
		}
		return nil, err
	}

	for _, plugin := range req.plugins {
		if err := mattermost.InstallPlugin(ctx, plugin.path, plugin.id, plugin.config); err != nil {
			if err2 := mattermost.Terminate(ctx); err2 != nil {
				err = fmt.Errorf("%w + %w", err, err2)
			}
			return nil, err
		}
	}

	return mattermost, nil
}