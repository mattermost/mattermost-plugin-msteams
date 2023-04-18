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

This plugin supports the following features:
- Connect to MS Teams account using Device Code OAuth flow.

- Link Mattermost channels with MS Teams channels and sync messages between the linked channels.

- Link Mattermost DMs and group messages with Teams chats and sync messages.

- Sync Mattermost and MS Teams messages for any changes made in any existing messages on either side.

- Deletion of MS Teams messages is synced with Mattermost but it's not vice versa.

- - Sync posts containing markdown and attachments.

    ![image](https://user-images.githubusercontent.com/77336594/226587339-050c35da-a0f1-47db-a15f-f8d5f59bf8cd.png)
    ![image](https://user-images.githubusercontent.com/77336594/226587366-2c4231bc-1aa2-42c4-b692-bd4441c71c34.png)
    ![image](https://user-images.githubusercontent.com/77336594/226588263-a7915e4d-d9ae-4294-9134-326628febdfc.png)
    ![image](https://user-images.githubusercontent.com/77336594/226588309-3202b78f-d87d-439c-967b-25ba8ed328c9.png)

- Sync reactions on posts.

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

- **Connecting the bot account**
    - Run the slash command `/msteams-sync connect-bot` in any channel.
    - This command is visible and accessible by system admins only.
    - After running the slash command, you will get an ephemeral message from the MS Teams bot containing a link and a code to connect the bot account.
    - Click on that link and enter the code. If it asks for login, enter the Microsoft credentials for the dummy account and click `Continue` to authorize and connect the bot account.
    - Refer [here](./docs/azure_setup.md#step-2-create-a-user-account-to-act-as-a-bot) for more details on connecting the bot account.

    After connecting successfully, you will get an ephemeral message from the MS Teams bot saying "The bot account has been connected".
