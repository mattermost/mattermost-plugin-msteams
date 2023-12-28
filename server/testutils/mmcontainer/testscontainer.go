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

type LogConsumer struct{}

func (l *LogConsumer) Accept(entry testcontainers.Log) {
	fmt.Println(string(entry.Content))
}

// MattermostContainer represents the mattermost container type used in the module
type MattermostContainer struct {
	testcontainers.Container
	pgContainer *postgres.PostgresContainer
	network     *testcontainers.DockerNetwork
	username    string
	password    string
}

func (c *MattermostContainer) Url(ctx context.Context) (string, error) {
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
	url, err := c.Url(ctx)
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

func (c *MattermostContainer) initData(ctx context.Context, env map[string]string) {
	email := defaultEmail
	if env["TC_USER_EMAIL"] != "" {
		email = env["TC_USER_EMAIL"]
	}
	c.username = defaultUsername
	if env["TC_USER_USERNAME"] != "" {
		c.username = env["TC_USER_USERNAME"]
	}
	c.password = defaultPassword
	if env["TC_USER_PASSWORD"] != "" {
		c.password = env["TC_USER_PASSWORD"]
	}
	c.CreateAdmin(ctx, email, c.username, c.password)

	teamName := defaultTeamName
	if env["TC_TEAM_NAME"] != "" {
		teamName = env["TC_TEAM_NAME"]
	}
	teamDisplayName := defaultTeamDisplayName
	if env["TC_TEAM_DISPLAY_NAME"] != "" {
		teamDisplayName = env["TC_TEAM_DISPLAY_NAME"]
	}
	c.CreateTeam(ctx, teamName, teamDisplayName)

	c.AddUserToTeam(ctx, c.username, teamName)
}

func (c *MattermostContainer) Terminate(ctx context.Context) error {
	var errors error
	if err := c.network.Remove(ctx); err != nil {
		errors = fmt.Errorf("%w + %w", errors, err)
	}

	if err := c.pgContainer.Terminate(ctx); err != nil {
		errors = fmt.Errorf("%w + %w", errors, err)
	}

	if err := c.Container.Terminate(ctx); err != nil {
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
	url, err := c.Url(ctx)
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

func (c *MattermostContainer) InstallPlugin(ctx context.Context, pluginPath string, pluginID string, configPath string) error {
	_, _, err := c.Exec(ctx, []string{"mmctl", "--local", "plugin", "add", pluginPath})
	if err != nil {
		return err
	}

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
func WithConfigFile(cfg string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
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
func WithInitScripts(scripts ...string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
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
func WithEnv(env, value string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
		req.Env[env] = value
	}
}

// WithAdmin sets the admin email, username and password for the mattermost container
func WithAdmin(email, username, password string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
		req.Env["TC_USER_EMAIL"] = email
		req.Env["TC_USER_USERNAME"] = username
		req.Env["TC_USER_PASSWORD"] = password
	}
}

// WithAdmin sets the admin email, username and password for the mattermost container
func WithTeam(teamName, teamDisplayName string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
		req.Env["TC_TEAM_NAME"] = teamName
		req.Env["TC_TEAM_DISPLAY_NAME"] = teamDisplayName
	}
}

// WithConfigFile sets the config file to be used for the postgres container
// It will also set the "config_file" parameter to the path of the config file
// as a command line argument to the container
func WithPlugin(pluginPath, pluginID string, pluginConfig map[string]any) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
		uuid, _ := uuid.NewUUID()

		f, err := os.CreateTemp("", "*.json")
		if err != nil {
			fmt.Println("Error creating the patch config file", err)
			return
		}

		patch := map[string]map[string]map[string]map[string]any{"PluginSettings": {"Plugins": {pluginID: pluginConfig}}}
		data, err := json.Marshal(patch)
		if err != nil {
			fmt.Println("Error marshaling the patch config", err)
			return
		}
		f.Write(data)
		f.Close()

		pluginFile := testcontainers.ContainerFile{
			HostFilePath:      pluginPath,
			ContainerFilePath: fmt.Sprintf("/tmp/%s.tar.gz", uuid.String()),
			FileMode:          0o755,
		}

		cfgFile := testcontainers.ContainerFile{
			HostFilePath:      f.Name(),
			ContainerFilePath: fmt.Sprintf("/tmp/%s.config.json", uuid.String()),
			FileMode:          0o755,
		}

		req.Files = append(req.Files, pluginFile, cfgFile)
		req.Env["TC_PLUGIN_PATH"] = fmt.Sprintf("/tmp/%s.tar.gz", uuid.String())
		req.Env["TC_PLUGIN_ID"] = pluginID
		req.Env["TC_PLUGIN_CONFIG"] = fmt.Sprintf("/tmp/%s.config.json", uuid.String())
		req.Env["TC_PLUGIN_CONFIG_LOCAL"] = f.Name()
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
func RunContainer(ctx context.Context, opts ...testcontainers.ContainerCustomizer) (*MattermostContainer, error) {
	newNetwork, err := network.New(ctx, network.WithCheckDuplicate())
	if err != nil {
		return nil, err
	}

	postgresContainer, err := runPostgresContainer(ctx, newNetwork)
	if err != nil {
		newNetwork.Remove(ctx)
		return nil, err
	}

	dbconn := fmt.Sprintf("postgres://user:pass@%s:%d/mattermost_test?sslmode=disable", "db", 5432)
	req := testcontainers.ContainerRequest{
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
	}

	genericContainerReq := testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	}

	for _, opt := range opts {
		opt.Customize(&genericContainerReq)
	}

	container, err := testcontainers.GenericContainer(ctx, genericContainerReq)
	if err != nil {
		if err2 := newNetwork.Remove(ctx); err2 != nil {
			err = fmt.Errorf("%w + %w", err, err2)
		}
		if err2 := postgresContainer.Terminate(ctx); err2 != nil {
			err = fmt.Errorf("%w + %w", err, err2)
		}
		return nil, err
	}

	mattermost := &MattermostContainer{Container: container, pgContainer: postgresContainer, network: newNetwork}
	mattermost.setSiteURL(context.Background())

	mattermost.initData(ctx, req.Env)

	if req.Env["TC_PLUGIN_PATH"] != "" && req.Env["TC_PLUGIN_ID"] != "" && req.Env["TC_PLUGIN_CONFIG"] != "" {
		mattermost.InstallPlugin(ctx, req.Env["TC_PLUGIN_PATH"], req.Env["TC_PLUGIN_ID"], req.Env["TC_PLUGIN_CONFIG"])
		os.Remove(req.Env["TC_PLUGIN_CONFIG_LOCAL"])
	}

	return mattermost, nil
}
