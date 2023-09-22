# Mattermost MS Teams Sync Plugin
## Table of Contents
- [License](#license)
- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
- [Setup](#setup)
- [Connecting to MS Teams](#connecting-to-ms-teams)
- [FAQs](#faqs)

## License

See the [LICENSE](./LICENSE) file for license rights and limitations.

## Overview

This plugin integrates MS Teams with Mattermost by providing automated syncing of messages from Mattermost to MS Teams and vice versa. For a stable production release, please download the latest version from the Plugin Marketplace and follow the instructions to [install](#installation) and [configure](#setup) the plugin. If you are a developer who wants to work on this plugin, please switch to the [Developer docs](./docs/developer_docs.md).

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

## Installation

1. Go to the releases page of this GitHub repository and download the latest release for your Mattermost server: https://github.com/mattermost/mattermost-plugin-msteams-sync/releases
2. Upload this file on the Mattermost **System Console > Plugins > Management** page to install the plugin. To learn more about how to upload a plugin, [see the documentation](https://docs.mattermost.com/administration/plugins.html#plugin-uploads).
3. Enable the plugin from **System Console > Plugins > MSTeams Sync**.

## Setup

- [Microsoft Azure Setup](./docs/azure_setup.md)
- [Plugin Setup](./docs/plugin_setup.md)

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
    - After running the slash command, you will get an ephemeral message from the MS Teams bot containing a link to connect the bot account.
    - Click on that link. If it asks for login, enter the Microsoft credentials for the dummy account created following [these steps](./docs/azure_setup.md#step-2-create-a-user-account-to-act-as-a-bot).
    - Refer [here](./docs/azure_setup.md#step-2-create-a-user-account-to-act-as-a-bot) for more details.

## Slash commands

- `/msteams-sync connect` :- This is used to connect your Mattermost account to MS Teams account.
- `/msteams-sync disconnect` :- This is used to disconnect your Mattermost account from MS Teams account.
- `/msteams-sync link` :- This is used to link the currently active Mattermost channel with an MS Teams channel and it can only be run by users who are channel admins and above. To run this command, you must have your Mattermost account connected with MS Teams. This command takes two arguments - MS Teams team ID and channel ID which you can get from command autocomplete.
- `/msteams-sync unlink` :- This is used to unlink the currently active Mattermost channel with the MS Teams channel and it can only be run by users who are channel admins and above. To run this command, you don't need to have your Mattermost account connected to MS Teams.
- `/msteams-sync show` :- This is used to show the link of the currently active Mattermost channel. It displays the team name, team ID, channel name and channel ID of MS Teams to which the currently active MM channel is linked.

### System admins only
- `/msteams-sync connect-bot` :- This is used to connect the bot account in Mattermost to an account in MS Teams and it can only be run by system admins.
- `/msteams-sync disconnect-bot` :- This is used to disconnect the bot account in Mattermost from the MS Teams account and it can only be run by system admins.
- `/msteams-sync show-links` :- This is used to show all the currently active links and can only be run by system admins. It displays all the links that contain Mattermost team, Mattermost channel, MS Teams team, MS Teams channel.
- `/msteams-sync promote` :- This is used to promote a synthetic user to a normal user and can only be run by system admins. This command takes two parameters i.e. current_username and the new_username. **Note that after promoting the user, he will be counted under the Mattermost license**.

## FAQs
    - Read about the FAQs [here](./docs/faqs.md)
    
