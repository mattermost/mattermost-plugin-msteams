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
    - **Sync guest users**: Enabling this configuration will sync the MS Teams guest users on Mattermost.
    - **Sync direct and group messages**: Enable this for enabling the syncing of direct and group messages.
    - **Enabled Teams**: This config is for the Mattermost teams for which syncing is enabled. Enter a comma-separated list of Mattermost team names. If you leave it empty, it will enable syncing for all the teams. 

    ![image](https://github.com/mattermost/mattermost-plugin-msteams-sync/assets/100013900/077058f2-cd59-4287-a85d-12e6d158d208)
