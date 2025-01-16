# Mattermost Microsoft Teams Plugin

The plugin enables you to break through siloes in a mixed Mattermost and Microsoft Teams environment by forwarding real-time chat notifications from Microsoft Teams to Mattermost.

See the [Mattermost Product Documentation](https://docs.mattermost.com/integrate/ms-teams-interoperability.html) for details on installing, configuring, enabling, and using this Mattermost integration. If you are a developer who wants to work on this plugin, visit the [Development](#development) section on this page

## Features

- Connect to a Microsoft Teams account using the OAuth2 flow.
- Send bot notifications in Mattermost for chat and group chat messages received in Microsoft Teams. To respond, select the link provided to open the chat in Microsoft Teams.
- Attachments sent from Microsoft Teams will be forwarded to Mattermost.

## License

This repository is licensed under the [Mattermost Source Available License](LICENSE.txt) and requires a valid Enterprise Edition License when used for production. See [frequently asked questions](https://docs.mattermost.com/overview/faq.html#mattermost-source-available-license) to learn more.

Although a valid Mattermost Enterprise Edition License is required if using this plugin in production, the [Mattermost Source Available License](LICENSE.txt) allows you to compile and test this plugin in development and testing environments without a Mattermost Enterprise Edition License. As such, we welcome community contributions to this plugin.

If you're running an Enterprise Edition of Mattermost and don't already have a valid license, you can obtain a trial license from **System Console > Edition and License**. If you're running the Team Edition of Mattermost, including when you run the server directly from source, you may instead configure your server to enable both testing (`ServiceSettings.EnableTesting`) and developer mode (`ServiceSettings.EnableDeveloper`). These settings are not recommended in production environments.

## Development

### Setup

Make sure you have the following components installed:  

- Go - v1.18 - [Getting Started](https://golang.org/doc/install)
    > **Note:** If you have installed Go to a custom location, make sure the `$GOROOT` variable is set properly. Refer [Installing to a custom location](https://golang.org/doc/install#install).

- Make

You also want to have the environment variable `MM_SERVICESETTINGS_ENABLEDEVELOPER="true"` set if you are not working on linux. Without this, the plugin will be built excusively for linux.

In your mattermost config, make sure that `PluginSettings.EnableUploads` is `true`, and `FileSettings.MaxFileSize` is large enough to accept the plugin bundle (eg `256000000`)

### Building the plugin

Run the following command in the plugin repo to prepare a compiled, distributable plugin zip:

```bash
make dist
```

After a successful build, a `.tar.gz` file in the `/dist` folder will be created which can be uploaded to Mattermost. To avoid having to manually install your plugin, deploy your plugin using one of the following options.

### Deploying with Local Mode

If your Mattermost server is running locally, you can enable [local mode](https://docs.mattermost.com/administration/mmctl-cli-tool.html#local-mode) to streamline deploying your plugin. Edit your server configuration as follows:

```
{
    "ServiceSettings": {
        ...
        "EnableLocalMode": true,
        "LocalModeSocketLocation": "/var/tmp/mattermost_local.socket"
    }
}
```

and then deploy your plugin:

```bash
make deploy
```

You may also customize the Unix socket path:

```bash
export MM_LOCALSOCKETPATH=/var/tmp/alternate_local.socket
make deploy
```

If developing a plugin with a web app, watch for changes and deploy those automatically:

```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_TOKEN=j44acwd8obn78cdcx7koid4jkr
make watch
```

### Deploying with credentials

Alternatively, you can authenticate with the server's API with credentials:

```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_USERNAME=admin
export MM_ADMIN_PASSWORD=password
make deploy
```

or with a [personal access token](https://docs.mattermost.com/developer/personal-access-tokens.html):

```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_TOKEN=j44acwd8obn78cdcx7koid4jkr
make deploy
```

## How to Release

To trigger a release, follow these steps:

1. **For Patch Release:** Run the following command:

    ```
    make patch
    ```

   This will release a patch change.

2. **For Minor Release:** Run the following command:

    ```
    make minor
    ```

   This will release a minor change.

3. **For Major Release:** Run the following command:

    ```
    make major
    ```

   This will release a major change.

4. **For Patch Release Candidate (RC):** Run the following command:

    ```
    make patch-rc
    ```

   This will release a patch release candidate.

5. **For Minor Release Candidate (RC):** Run the following command:

    ```
    make minor-rc
    ```

   This will release a minor release candidate.

6. **For Major Release Candidate (RC):** Run the following command:

    ```
    make major-rc
    ```

   This will release a major release candidate.
