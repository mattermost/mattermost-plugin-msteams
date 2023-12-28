import {StartedTestContainer, GenericContainer, StartedNetwork, Network, Wait} from "testcontainers";
import {StartedPostgreSqlContainer, PostgreSqlContainer} from "@testcontainers/postgresql";
import { v4 as uuidv4 } from 'uuid';

const defaultEmail           = "admin@example.com";
const defaultUsername        = "admin";
const defaultPassword        = "admin";
const defaultTeamName        = "test";
const defaultTeamDisplayName = "Test";
const defaultMattermostImage = "mattermost/mattermost-enterprise-edition";

// MattermostContainer represents the mattermost container type used in the module
export default class MattermostContainer {
    container: StartedTestContainer;
    pgContainer: StartedPostgreSqlContainer;
    network:     StartedNetwork;
    email: string;
    username:    string;
	password:    string;
    teamName: string;
    teamDisplayName: string;
    envs:        {[key: string]: string};
    command:    string[];
    files: any[];
    filesContent: any[];
    configFile: any[];
    plugins: any[];

    url(): string {
        const containerPort = this.container.getMappedPort(8065)
        const host = this.container.getHost()
        return `http://${host}:${containerPort}`
    }

    // getAdminClient(): Client4 {
    //     const url = this.url()
    //     const client = new Client4(url)
    //     client.Login(this.username, this.password)
    //     return client
    // }

    stop = async () => {
        await this.pgContainer.stop()
        await this.container.stop()
        await this.network.stop()
    }
    createAdmin = (email: string, username: string, password: string): void => {
        this.container.exec(["mmctl", "--local", "user", "create", "--email", email, "--username", username, "--password", password, "--system-admin", "--email-verified"])
    }

    createUser = (email: string, username: string, password: string): void => {
        this.container.exec(["mmctl", "--local", "user", "create", "--email", email, "--username", username, "--password", password, "--email-verified"])
    }

    createTeam = (name: string, displayName: string): void => {
        this.container.exec(["mmctl", "--local", "team", "create", "--name", name, "--display-name", displayName])
    }

    addUserToTeam = (username: string, teamname: string): void => {
        this.container.exec(["mmctl", "--local", "team", "users", "add", teamname, username])
    }

    getLogs = async (lines: number): Promise<string> => {
        const {output} = await this.container.exec(["mmctl", "--local", "logs", "--number", lines.toString()])
        return output
    }

    setSiteURL = (): void => {
        const url = this.url()
        this.container.exec(["mmctl", "--local", "config", "set", "ServiceSettings.SiteURL", url])
        const containerPort = this.container.getMappedPort(8065)
        this.container.exec(["mmctl", "--local", "config", "set", "ServiceSettings.ListenAddress", `${containerPort}`])
    }

    installPlugin = (pluginPath: string, pluginID: string, configPath: string): void => {
        this.container.exec(["mmctl", "--local", "plugin", "add", pluginPath])
        this.container.exec(["mmctl", "--local", "config", "patch", configPath])
        this.container.exec(["mmctl", "--local", "plugin", "enable", pluginID])
    }

    withEnv = (env: string, value: string): MattermostContainer => {
        this.envs[env] = value
        return this
    }

    withAdmin = (email: string, username: string, password: string): MattermostContainer => {
        this.email = email;
        this.username = username;
        this.password = password;
        return this;
    }

    withTeam = (teamName: string, teamDisplayName: string): MattermostContainer => {
        this.teamName = teamName;
        this.teamDisplayName = teamDisplayName;
        return this;
    }

    withConfigFile = (cfg: string): MattermostContainer => {
        const cfgFile = {
            source: cfg,
            target: "/etc/mattermost.json",
        }
        this.configFile.push(cfgFile)
        this.command.push("-c", "/etc/mattermost.json")
        return this
    }

    withPlugin = (pluginPath: string, pluginID: string, pluginConfig: string): MattermostContainer => {
        const uuid = uuidv4();

		const patch = `{"PluginSettings": {"Plugins": {"${pluginID}": ${JSON.stringify(pluginConfig)}}}}`

        this.files.push({
			source:      pluginPath,
            target:      `/tmp/${uuid}.tar.gz`,
		})

        this.filesContent.push({
            content: patch,
            target: `/tmp/${uuid}.config.json`,
		})

        this.plugins.push({id: pluginID, path: `/tmp/${uuid}.tar.gz`, config: `/tmp/${uuid}.config.json`})

        return this
    }

    constructor() {
        this.command = ["mattermost", "server"];
        const dbconn = `postgres://user:pass@db:5432/mattermost_test?sslmode=disable`;
        this.envs = {
                "MM_SQLSETTINGS_DATASOURCE":          dbconn,
                "MM_SQLSETTINGS_DRIVERNAME":          "postgres",
                "MM_SERVICESETTINGS_ENABLELOCALMODE": "true",
                "MM_PASSWORDSETTINGS_MINIMUMLENGTH":  "5",
                "MM_PLUGINSETTINGS_ENABLEUPLOADS":    "true",
                "MM_FILESETTINGS_MAXFILESIZE":        "256000000",
                "MM_LOGSETTINGS_CONSOLELEVEL":        "DEBUG",
                "MM_LOGSETTINGS_FILELEVEL":           "DEBUG",
        };
        this.email = defaultEmail;
        this.username = defaultUsername;
        this.password = defaultPassword;
        this.teamName = defaultTeamName;
        this.teamDisplayName = defaultTeamDisplayName;
        this.files = [];
        this.filesContent = [];
        this.plugins = [];
        this.configFile = [];
    }



    start = async (): Promise<MattermostContainer> => {
        this.network = await new Network().start()
        this.pgContainer = await new PostgreSqlContainer("docker.io/postgres:15.2-alpine")
            .withDatabase("mattermost_test")
            .withUsername("user")
            .withPassword("pass")
            .withNetworkMode(this.network.getName())
            .withWaitStrategy(Wait.forLogMessage("database system is ready to accept connections"))
            .withNetworkAliases("db")
            .start()

        this.container = await new GenericContainer(defaultMattermostImage)
            .withEnvironment(this.envs)
            .withExposedPorts(8065)
            .withNetwork(this.network)
            .withNetworkAliases("mattermost")
            .withCommand(this.command)
            .withWaitStrategy(Wait.forLogMessage("Server is listening on"))
            .withCopyFilesToContainer(this.files)
            .withCopyFilesToContainer(this.configFile)
            .withCopyContentToContainer(this.filesContent)
            .start()

        this.setSiteURL()
        this.createAdmin(this.email, this.username, this.password)
        this.createTeam(this.teamName, this.teamDisplayName)
        this.addUserToTeam(this.username, this.teamName)

        for (const plugin of this.plugins) {
            this.installPlugin(plugin.path, plugin.id, plugin.config)
        }

        return this
    }
}




