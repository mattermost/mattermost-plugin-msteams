# Mattermost MS Teams Connector Plugin
## Table of Contents
- [License](#license)
- [Overview](#overview)
- [Features](#features)
- [Basic Knowledge](#basic-knowledge)
- [Installation](#installation)
- [Setup](#setup)
- [Connecting to MS Teams](#connecting-to-ms-teams)
- [Development](#development)


## License

See the [LICENSE](../LICENSE) file for license rights and limitations.

## Overview

This plugin integrates MS Teams with Mattermost by providing automated syncing of messages from Mattermost to MS Teams and vice versa. For a stable production release, please download the latest version from the Plugin Marketplace and follow the instructions to [install](#installation) and [configure](#setup) the plugin.

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

## Basic Knowledge

- [What is Microsoft Graph?](https://learn.microsoft.com/en-us/graph/overview)
- [What is Microsoft Graph API?](https://learn.microsoft.com/en-us/graph/use-the-api)  
    - [Graph API explorer](https://developer.microsoft.com/en-us/graph/graph-explorer)
- [What are subscriptions in Microsoft Graph?](https://learn.microsoft.com/en-us/graph/api/resources/subscription?view=graph-rest-1.0)
- [What are change notifications present in Microsoft for messages?](https://learn.microsoft.com/en-us/graph/teams-changenotifications-chatmessage)
- [What are lifecycle notifications?](https://learn.microsoft.com/en-us/graph/webhooks-lifecycle)

## Installation

1. Go to the [releases page of this GitHub repository](github.com/mattermost/mattermost-plugin-msteams-sync/releases) and download the latest release for your Mattermost server.
2. Upload this file on the Mattermost **System Console > Plugins > Management** page to install the plugin. To learn more about how to upload a plugin, [see the documentation](https://docs.mattermost.com/administration/plugins.html#plugin-uploads).
3. Enable the plugin from **System Console > Plugins > MS Teams Connector**.

## Setup

- [Microsoft Azure Setup](./azure_setup.md)
- [Plugin Setup](./plugin_setup.md)


## Connecting to MS Teams

There are two methods by which you can connect your Mattermost account to your MS Teams account.

- **Using slash command**
    - Run the slash command `/msteams-sync connect` in any channel.

    - You will get an ephemeral message from the MS Teams bot containing a link to connect your account.

    - Click on that link. If it asks for login, enter your Microsoft credentials to connect your account.

- **Using the button in the full screen modal**
    - If the setting "Enforce connected accounts" is enabled in the plugin's config settings, then a full screen modal appears that looks like this - 
    
    ![image](https://github.com/mattermost/mattermost-plugin-msteams-sync/assets/100013900/ced5e65b-a52a-46f4-a7fa-dac6e2ff8440)

    - Click on the "Connect account" button.

    - A window appears containing a link to connect your account.

    - Open that link. If it asks for login, enter your Microsoft credentials to connect your account.

- **Connecting the bot account**
    - Run the slash command `/msteams-sync connect-bot` in any channel.
    - This command is visible and accessible by system admins only.
    - After running the slash command, you will get an ephemeral message from the MS Teams bot containing a link to connect your account.
    - Click on that link. If it asks for login, enter your Microsoft credentials to connect your account.
    - Refer [here](./docs/azure_setup.md#step-2-create-a-user-account-to-act-as-a-bot) for more details on connecting the bot account.

## Development

### Setup

Make sure you have the following components installed:  

- Go - v1.18 - [Getting Started](https://golang.org/doc/install)
    > **Note:** If you have installed Go to a custom location, make sure the `$GOROOT` variable is set properly. Refer [Installing to a custom location](https://golang.org/doc/install#install).

- Make

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
