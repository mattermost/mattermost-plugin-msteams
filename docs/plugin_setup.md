# Configuration

- Go to the MS Teams sync plugin configuration page on Mattermost as **System Console > Plugins > MSTeams Sync**.
- On the MS Teams plugin configuration page, you need to configure the following:
    - **Tenant ID**: Enter the Tenant ID by copying it from the Azure portal.
    - **Client ID**: Enter the Client ID of your registered OAuth app in Azure portal.
    - **Client Secret**: Enter the client secret of your registered OAuth app in Azure portal.
    - **At Rest Encryption Key**: Regenerate a new encryption secret. This encryption secret will be used to encrypt and decrypt the OAuth token.
    - **Webhook secret**: Regenerate a new webhook secret.
    - **Bot Username**: The username for the bot account.
    - **Bot Password**: The password for the bot account.
    - **Enforce connected accounts**: Enabling this will enforce all the users to connect their Mattermost account to their MS Teams account.
    - **Allow to temporarily skip connect user**: Enabling this will allow the users to temporarily skip connecting their accounts.
    - **Sync users**: This config is for the interval (in minutes) in which the users will be synced between Mattermost and MS Teams.
    - **Sync direct and group messages**: Enable this for enabling the syncing of direct and group messages.

    ![image](https://user-images.githubusercontent.com/77336594/226391146-9f760abd-4edc-461f-9582-095acc822eb6.png)

