import {StartedTestContainer, GenericContainer, StartedNetwork, Network, Wait} from "testcontainers";
import {StartedPostgreSqlContainer, PostgreSqlContainer} from "@testcontainers/postgresql";
import {Client4} from "@mattermost/client";
import { Client } from 'pg'

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
    configFile: any[];
    plugins: any[];

    url(): string {
        const containerPort = this.container.getMappedPort(8065)
        const host = this.container.getHost()
        return `http://${host}:${containerPort}`
    }

    db = async (): Client => {
        const port = this.pgContainer.getMappedPort(5432)
        const host = this.pgContainer.getHost()
        const database = "mattermost_test"
        const client = new Client({user: "user", password: "pass", host, port, database})
        await client.connect()
        return client
    }

    getAdminClient = async (): Promise<Client4> => {
        return this.getClient(this.username, this.password)
    }

    getClient = async (username: string, password: string): Promise<Client4> => {
        const url = this.url()
        const client = new Client4()
        client.setUrl(url)
        await client.login(username, password)
        return client
    }

    stop = async () => {
        await this.pgContainer.stop()
        await this.container.stop()
        await this.network.stop()
    }

    createAdmin = async (email: string, username: string, password: string) => {
        await this.container.exec(["mmctl", "--local", "user", "create", "--email", email, "--username", username, "--password", password, "--system-admin", "--email-verified"])
    }

    createUser = async (email: string, username: string, password: string) => {
        await this.container.exec(["mmctl", "--local", "user", "create", "--email", email, "--username", username, "--password", password, "--email-verified"])
    }

    createTeam = async (name: string, displayName: string) => {
        await this.container.exec(["mmctl", "--local", "team", "create", "--name", name, "--display-name", displayName])
    }

    addUserToTeam = async (username: string, teamname: string) => {
        await this.container.exec(["mmctl", "--local", "team", "users", "add", teamname, username])
    }

    getLogs = async (lines: number): Promise<string> => {
        const {output} = await this.container.exec(["mmctl", "--local", "logs", "--number", lines.toString()])
        return output
    }

    setSiteURL = async () => {
        const url = this.url()
        await this.container.exec(["mmctl", "--local", "config", "set", "ServiceSettings.SiteURL", url])
        const containerPort = this.container.getMappedPort(8065)
        await this.container.exec(["mmctl", "--local", "config", "set", "ServiceSettings.ListenAddress", `${containerPort}`])
    }

    installPlugin = async (pluginPath: string, pluginID: string, pluginConfig: any) => {
		const patch = JSON.stringify({PluginSettings: {Plugins: {[pluginID]: pluginConfig}}})

        await this.container.copyFilesToContainer([{source: pluginPath, target: `/tmp/plugin.tar.gz`}])
        await this.container.copyContentToContainer([{content: patch, target: `/tmp/plugin.config.json`}])

        await this.container.exec(["mmctl", "--local", "plugin", "add", '/tmp/plugin.tar.gz'])
        await this.container.exec(["mmctl", "--local", "config", "patch", '/tmp/plugin.config.json'])
        await this.container.exec(["mmctl", "--local", "plugin", "enable", pluginID])
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

    withPlugin = (pluginPath: string, pluginID: string, pluginConfig: any): MattermostContainer => {
        this.plugins.push({id: pluginID, path: pluginPath, config: pluginConfig})

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
        this.plugins = [];
        this.configFile = [];
    }

    start = async (): Promise<MattermostContainer> => {
        this.network = await new Network().start()
        this.pgContainer = await new PostgreSqlContainer("docker.io/postgres:15.2-alpine")
            .withExposedPorts(5432)
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
            .withCopyFilesToContainer(this.configFile)
            .start()

        await this.setSiteURL()
        await this.createAdmin(this.email, this.username, this.password)
        await this.createTeam(this.teamName, this.teamDisplayName)
        await this.addUserToTeam(this.username, this.teamName)

        for (const plugin of this.plugins) {
            await this.installPlugin(plugin.path, plugin.id, plugin.config)
        }

        return this
    }
}
