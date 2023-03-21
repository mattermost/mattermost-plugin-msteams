# Configuration

- Go to the MS Teams sync plugin configuration page on Mattermost as **System Console > Plugins > MSTeams Sync**.
- On the MS Teams plugin configuration page, you need to configure the following:
    - **Tenant ID**: Enter the Tenant ID by copying it from the Azure portal.
    - **Client ID**: Enter the Client ID of your registered OAuth app in Azure portal.
    - **Client Secret**: Enter the client secret of your registered OAuth app in Azure portal.
    - **At Rest Encryption Key**: Regenerate a new encryption secret. This encryption secret will be used to encrypt and decrypt the OAuth token.
    - **Webhook secret**: Regenerate a new webhook secret.
    - **Use the evalution API pay model**: Enable this only for testing purposes. You need the pay model to be able to support enough messages notifications to work in a real world scenario.
    - **Enforce connected accounts**: Enabling this will enforce all the users to connect their Mattermost account to their MS Teams account.
    - **Allow to temporarily skip connect user**: Enabling this will allow the users to temporarily skip connecting their accounts.
    - **Sync users**: This config is for the interval (in minutes) in which the users will be synced between Mattermost and MS Teams, if you set it to empty it will not syncrhonize the users.
    - **Sync direct and group messages**: Enable this for enabling the syncing of direct and group messages.

    ![image](https://user-images.githubusercontent.com/77336594/226593804-e3245221-9fdc-456e-aefb-be243c967394.png)

- Go to your any channel in Mattermost as a system admin and run the command `/msteams-sync connect-bot` to connect the bot account to the previously created bot account in MS Teams.
