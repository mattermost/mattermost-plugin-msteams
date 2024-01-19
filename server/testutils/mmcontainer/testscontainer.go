package mmcontainer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
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
	dbconn                 = "postgres://user:pass@db:5432/mattermost_test?sslmode=disable"
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
	config          *model.Config
	logConsumer     testcontainers.LogConsumer
}

// MattermostContainer represents the mattermost container type used in the module
type MattermostContainer struct {
	testcontainers.Container
	pgContainer *postgres.PostgresContainer
	network     *testcontainers.DockerNetwork
	username    string
	password    string
}

// URL returns the url of the mattermost instance
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

// GetClient returns a mattermost client with the admin logged in for the mattermost instance
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

// PostgresConnection returns a direct sql.DB postgres connection to the postgres container
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

// PostgresDSN returns the public postgres dsn for the postgres container
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

// Terminate terminates the mattermost and postgres containers
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

// CreateAdmin creates an admin user
func (c *MattermostContainer) CreateAdmin(ctx context.Context, email, username, password string) error {
	_, err := c.RunCtlCommand(ctx, "user", []string{"create", "--email", email, "--username", username, "--password", password, "--system-admin", "--email-verified"})
	return err
}

// CreateUser creates a regular user
func (c *MattermostContainer) CreateUser(ctx context.Context, email, username, password string) error {
	_, err := c.RunCtlCommand(ctx, "user", []string{"create", "--email", email, "--username", username, "--password", password, "--email-verified"})
	return err
}

// CreateTeam creates a team
func (c *MattermostContainer) CreateTeam(ctx context.Context, name, displayName string) error {
	_, err := c.RunCtlCommand(ctx, "team", []string{"create", "--name", name, "--display-name", displayName})
	return err
}

// AddUserToTeam adds a user to a team
func (c *MattermostContainer) AddUserToTeam(ctx context.Context, username, teamname string) error {
	_, err := c.RunCtlCommand(ctx, "team", []string{"users", "add", teamname, username})
	return err
}

// GetLogs returns the logs of the mattermost instance
func (c *MattermostContainer) GetLogs(ctx context.Context, lines int) (string, error) {
	output, err := c.RunCtlCommand(ctx, "logs", []string{"--number", fmt.Sprintf("%d", lines)})
	if err != nil {
		return "", err
	}
	outputData, err := io.ReadAll(output)
	if err != nil {
		return "", err
	}
	return string(outputData), nil
}

// SetConfig sets the config key to the given value
func (c *MattermostContainer) SetConfig(ctx context.Context, configKey string, configValue string) error {
	_, err := c.RunCtlCommand(ctx, "config", []string{"set", configKey, configValue})
	return err
}

// setSiteURL sets the site url and listen address to the mattermost instance
func (c *MattermostContainer) setSiteURL(ctx context.Context) error {
	url, err := c.URL(ctx)
	if err != nil {
		return err
	}
	containerPort, err := c.MappedPort(ctx, "8065/tcp")
	if err != nil {
		return err
	}

	if err = c.SetConfig(ctx, "ServiceSettings.SiteURL", url); err != nil {
		return err
	}

	if err = c.SetConfig(ctx, "ServiceSettings.ListenAddress", fmt.Sprintf(":%d", containerPort.Int())); err != nil {
		return err
	}

	return nil
}

// UpdateConfig updates the config to be used for the mattermost instance
func (c *MattermostContainer) UpdateConfig(ctx context.Context, cfg *model.Config) error {
	config, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	configPath := "/tmp/new-config.json"
	err = c.CopyToContainer(ctx, config, configPath, 0o755)
	if err != nil {
		return err
	}
	_, err = c.RunCtlCommand(ctx, "config", []string{"patch", configPath})
	return err
}

// RunCtlCommand runs the mmctl command with the given arguments
func (c *MattermostContainer) RunCtlCommand(ctx context.Context, command string, args []string) (io.Reader, error) {
	exitCode, output, err := c.Exec(ctx, append([]string{"mmctl", "--local", command}, args...))
	if err != nil {
		return nil, err
	}
	if exitCode != 0 {
		outputData, err := io.ReadAll(output)
		if err != nil {
			outputData = []byte{}
		}
		return nil, fmt.Errorf("exit code %d\noutput:\n%s", exitCode, string(outputData))
	}
	return output, nil
}

// InstallPlugin installs a plugin in the mattermost instance
func (c *MattermostContainer) InstallPlugin(ctx context.Context, pluginPath string, pluginID string, pluginConfig map[string]any) error {
	patch := map[string]map[string]map[string]map[string]any{"PluginSettings": {"Plugins": {pluginID: pluginConfig}}}
	config, err := json.Marshal(patch)
	if err != nil {
		return err
	}

	if _, err = c.RunCtlCommand(ctx, "plugin", []string{"add", pluginPath}); err != nil {
		return err
	}

	configPath := "/tmp/plugin-config-" + pluginID + ".json"
	err = c.CopyToContainer(ctx, config, configPath, 0o755)
	if err != nil {
		return err
	}

	if _, err = c.RunCtlCommand(ctx, "config", []string{"patch", configPath}); err != nil {
		return err
	}

	if _, err = c.RunCtlCommand(ctx, "plugin", []string{"enable", pluginID}); err != nil {
		return err
	}
	return nil
}

// init initializes the mattermost instance
func (c *MattermostContainer) init(ctx context.Context, req MattermostContainerRequest) error {
	if req.config != nil {
		if err := c.UpdateConfig(ctx, req.config); err != nil {
			return err
		}
	}

	if err := c.setSiteURL(context.Background()); err != nil {
		return err
	}

	if err := c.CreateAdmin(ctx, req.email, req.username, req.password); err != nil {
		return err
	}

	if err := c.CreateTeam(ctx, req.teamName, req.teamDisplayName); err != nil {
		return err
	}

	if err := c.AddUserToTeam(ctx, req.username, req.teamName); err != nil {
		return err
	}

	for _, plugin := range req.plugins {
		if err := c.InstallPlugin(ctx, plugin.path, plugin.id, plugin.config); err != nil {
			return err
		}
	}
	return nil
}

// WithConfigFile sets the config file to be used for the mattermost instance
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

// WithConfig updates the config to be used for the mattermost instance after the start up
func WithConfig(cfg *model.Config) MattermostCustomizeRequestOption {
	return func(req *MattermostContainerRequest) {
		req.config = cfg
	}
}

// WithEnv sets the environment variable to the given value
func WithEnv(env, value string) MattermostCustomizeRequestOption {
	return func(req *MattermostContainerRequest) {
		req.Env[env] = value
	}
}

// WithAdmin sets the admin email, username and password for the mattermost instance
func WithAdmin(email, username, password string) MattermostCustomizeRequestOption {
	return func(req *MattermostContainerRequest) {
		req.email = email
		req.username = username
		req.password = password
	}
}

// WithTeam sets the initial team name and display name for the mattermost instance
func WithTeam(teamName, teamDisplayName string) MattermostCustomizeRequestOption {
	return func(req *MattermostContainerRequest) {
		req.teamName = teamName
		req.teamDisplayName = teamDisplayName
	}
}

// WithPlugin sets the plugin to be installed in the mattermost instance
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

type tLogConsumer struct {
	t *testing.T
}

func (tlc *tLogConsumer) Accept(log testcontainers.Log) {
	tlc.t.Log(string(log.Content))
}

// WithTestingLogConsumer pipes logs to the given testing instance.
func WithTestingLogConsumer(t *testing.T) MattermostCustomizeRequestOption {
	return func(req *MattermostContainerRequest) {
		req.logConsumer = &tLogConsumer{t}
	}
}

// runPostgresContainer creates a postgres container
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

// RunContainer creates an instance of the mattermost container type
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
					"MM_LOGSETTINGS_ENABLEFILE":          "true",
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

	if req.logConsumer != nil {
		container.FollowOutput(req.logConsumer)
	}

	mattermost := &MattermostContainer{
		Container:   container,
		pgContainer: postgresContainer,
		network:     newNetwork,
		username:    req.username,
		password:    req.password,
	}

	if err := mattermost.init(ctx, req); err != nil {
		if err2 := mattermost.Terminate(ctx); err2 != nil {
			err = fmt.Errorf("%w + %w", err, err2)
		}
		return nil, err
	}

	return mattermost, nil
}
