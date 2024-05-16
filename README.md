# Mattermost MS Teams Plugin

This plugin integrates MS Teams with Mattermost by providing automated syncing of messages from Mattermost to MS Teams and vice versa. For a stable production release, please download the latest version from the Plugin Marketplace and follow the instructions to [install](#installation) and [configure](#setup) the plugin. If you are a developer who wants to work on this plugin, visit the [Development](#development) section on this page

See the [Mattermost Product Documentation](https://docs.mattermost.com/integrate/ms-teams-interoperability.html) for details on installing, configuring, enabling, and using this Mattermost integration.

## Features

This plugin supports the following features:
- Connect to MS Teams account using the OAuth2 flow.
- Link Mattermost channels with MS Teams channels and sync messages between the linked channels.
- Link Mattermost DMs and group messages with Teams chats and sync messages.
- Sync Mattermost and MS Teams messages for any changes made in any existing messages on either side.
- Deletion of MS Teams messages is synced with Mattermost but it's not vice versa.
- Sync posts containing markdown and attachments.

    ![image](https://user-images.githubusercontent.com/77336594/226587339-050c35da-a0f1-47db-a15f-f8d5f59bf8cd.png)
    ![image](https://user-images.githubusercontent.com/77336594/226587366-2c4231bc-1aa2-42c4-b692-bd4441c71c34.png)
    ![image](https://user-images.githubusercontent.com/77336594/226588263-a7915e4d-d9ae-4294-9134-326628febdfc.png)
    ![image](https://user-images.githubusercontent.com/77336594/226588309-3202b78f-d87d-439c-967b-25ba8ed328c9.png)

- Sync reactions on posts.

## License

This repository is licensed under the [Mattermost Source Available License](LICENSE) and requires a valid Enterprise Edition License when used for production. See [frequently asked questions](https://docs.mattermost.com/overview/faq.html#mattermost-source-available-license) to learn more.

Although a valid Mattermost Enterprise Edition License is required if using this plugin in production, the [Mattermost Source Available License](LICENSE) allows you to compile and test this plugin in development and testing environments without a Mattermost Enterprise Edition License. As such, we welcome community contributions to this plugin.

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
