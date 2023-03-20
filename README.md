# Mattermost MS Teams Sync Plugin
## Table of Contents
- [License](#license)
- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
- [Setup](#setup)
- [Connecting to MS Teams](#connecting-to-ms-teams)

## License

See the [LICENSE](./LICENSE) file for license rights and limitations.

## Overview

This plugin integrates MS Teams with Mattermost by providing automated syncing of messages from Mattermost to MS Teams and vice versa. For a stable production release, please download the latest version from the Plugin Marketplace and follow the instructions to [install](#installation) and [configure](#setup) the plugin. If you are a developer who wants to work on this plugin, please switch to the [Developer docs](./docs/developer_docs.md).

## Features

This plugin contains the following features:
- Connecting/disconnecting to MS Teams account using Device Code OAuth flow.

- Linking of Mattermost channels with MS Teams channels and syncing of messages in the linked channels.

- Linking of Mattermost DMs and group messages with Teams chats and syncing of messages.

- Any updates done in Mattermost messages are synced with MS Teams messages and vice versa.

- Deletion of MS Teams messages is synced with Mattermost but it's not vice versa.

- Posts containing markdown and attachments are supported.

- Syncing of reactions on posts.

## Installation

1. Go to the [releases page of this GitHub repository](github.com/mattermost/mattermost-plugin-msteams-sync/releases) and download the latest release for your Mattermost server.
2. Upload this file on the Mattermost **System Console > Plugins > Management** page to install the plugin. To learn more about how to upload a plugin, [see the documentation](https://docs.mattermost.com/administration/plugins.html#plugin-uploads).
3. Enable the plugin from **System Console > Plugins > MSTeams Sync**.

## Setup

- [Microsoft Azure Setup](./docs/azure_setup.md)
- [Plugin Setup](./docs/plugin_setup.md)

## Connecting to MS Teams

There are two methods by which you can connect your Mattermost account to your MS Teams account.

- **Using slash command**
    - Run the slash command `/msteams-sync connect` in any channel.
    - You will get an ephemeral message from the MS Teams bot containing a link and a code to connect your account.
    - Click on that link and enter the code. If it asks for login, enter your Microsoft credentials and click `Continue` to authorize and connect your account.

- **Using the button in the full screen modal**
    - If the setting "Enforce connected accounts" is enabled in the plugin's config settings, then a full screen modal appears that looks like this - 
    
    ![image](https://user-images.githubusercontent.com/77336594/226347884-a6469d95-de68-4706-a145-9511d42bd7a4.png)

    - Click on the "Connect account" button. If it asks for login, enter your Microsoft credentials and click `Continue` to authorize and connect your account.

After connecting successfully, you will get an ephemeral message from the MS Teams bot saying "Your account has been connected".
