# Configuration

- Go to the MS Teams sync plugin configuration page on Mattermost as **System Console > Plugins > MSTeams Sync**.
- On the MS Teams plugin configuration page, you need to configure the following:
    - **Tenant ID**: Enter the Tenant ID by copying it from the Azure portal.
    - **Client ID**: Enter the Client ID of your registered OAuth app in Azure portal.
    - **Client Secret**: Enter the client secret of your registered OAuth app in Azure portal.
    - **At Rest Encryption Key**: Regenerate a new encryption secret. This encryption secret will be used to encrypt and decrypt the OAuth token.
    - **Webhook secret**: Regenerate a new webhook secret.
    - **Use the evalution API pay model**: Enable this only for testing purposes. You need the pay model to be able to support enough message notifications to work in a real world scenario.
    - **Enforce connected accounts**: Enabling this will enforce all the users to connect their Mattermost accounts to their MS Teams accounts.
    - **Allow to temporarily skip connect user**: Enabling this will allow the users to temporarily skip connecting their accounts.
    - **Sync users**: This config is for the interval (in minutes) in which the users will be synced between Mattermost and MS Teams. If you leave it empty, it will not syncrhonize the users.
    - **Sync guest users**: Enabling this configuration will sync the MS Teams guest users on Mattermost. Any existing active Mattermost user that was created by this plugin using the "Sync user" functionality corresponding to the MS Teams guest user will be deactivated if this setting is false, and vice versa.
    - **Sync direct and group messages**: Enable this for enabling the syncing of direct and group messages.
    - **Enabled Teams**: This config is for the Mattermost teams for which syncing is enabled. Enter a comma-separated list of Mattermost team names. If you leave it empty, it will enable syncing for all the teams.
    - **Prompt interval for DMs and GMs (in hours)**: This setting is for configuring the interval after which the user will get a prompt to connect their account when they try to post a message in a DM or GM without connecting their account. Leaving it empty will disable the prompt.
    - **Maximum size of attachments to support complete one time download (in MB)**: This setting is for configuring the maximum size of attachments that can be loaded into memory. Attachments bigger than this size will be streamed from MS Teams to Mattermost.
    - **Buffer size for streaming files (in MB)**: This setting is for configuring the buffer size for streaming files from MS Teams to Mattermost.

    ![image](https://github.com/mattermost/mattermost-plugin-msteams-sync/assets/100013900/e6a74693-1760-401f-bac2-83749c49fa2e)
