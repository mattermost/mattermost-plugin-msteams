# Configuration
In Mattermost, go to `System Console > Plugins > MSTeams Sync`. There, you will find the following `MS Teams sync` plugin configuration options.

|Option|Note|
|---|---|
Tenant ID | Enter the Tenant ID by copying it from the Azure portal.
Client ID | Enter the Client ID of your registered OAuth app in Azure portal.
Client Secret | Enter the client secret of your registered OAuth app in Azure portal.
At Rest Encryption Key | Regenerate a new encryption secret. This encryption secret will be used to encrypt and decrypt the OAuth token.
Webhook secret | Regenerate a new webhook secret.
Certificate Public | This configuration is for setting the public certificate for enabling certificate-based subscriptions on MS Graph.  
Certificate Key | This configuration is for setting the private key of the certificate for enabling certificate-based subscriptions. **Note**: For enabling certificate-based subscriptions, enter both the public part and private key of the certificate.
Use the evalution API pay model | Enable this only for testing purposes. You need the pay model to be able to support enough message notifications to work in a real world scenario.
Enforce connected accounts | Enabling this will enforce all the users to connect their Mattermost accounts to their MS Teams accounts.
Allow to temporarily skip connect user | Enabling this will allow the users to temporarily skip connecting their accounts.
Sync users | This config is for the interval (in minutes) in which the users will be synced between Mattermost and MS Teams. If you leave it empty, it will not syncrhonize the users.
Sync guest users | Enabling this configuration will sync the MS Teams guest users on Mattermost. Any existing active Mattermost user that was created by this plugin using the "Sync user" functionality corresponding to the MS Teams guest user will be deactivated if this setting is false, and vice versa.
Sync direct and group messages | Enable this for enabling the syncing of direct and group messages.
Enabled Teams | This config is for the Mattermost teams for which syncing is enabled. Enter a comma-separated list of Mattermost team names. If you leave it empty, it will enable syncing for all the teams.
Prompt interval for DMs and GMs (in hours) | This setting is for configuring the interval after which the user will get a prompt to connect their account when they try to post a message in a DM or GM without connecting their account. Leaving it empty will disable the prompt.
Maximum size of attachments to support complete one time download (in MB) | This setting is for configuring the maximum size of attachments that can be loaded into memory. Attachments bigger than this size will be streamed from MS Teams to Mattermost.
Buffer size for streaming files (in MB) | This setting is for configuring the buffer size for streaming files from MS Teams to Mattermost.
Max Connected Users | This setting sets the maximum number of users that may connect to their MS Teams account. Once connected, the user is added to a whitelist and may disconnect and reconnect at any time.
Automatically Promote Synthetic Users | This setting is for enabling the auto-promotion of synthetic users when they log in for the first time.
Disable using the sync msg infrastructure for tracking message changes | When true, the plugin will not enable any sync msg infrastructure.
Synthetic User Auth Service | This setting sets the authentication service to be used when creating or updating synthetic users. This should match the service used for member user access to Mattermost.
Synthetic User Auth Data | This setting sets the MS Teams user property to use as the authentication identifier. For AD/LDAP and SAML, the identifier's value should match the value provided by the ID attribute.

![image](https://github.com/mattermost/mattermost-plugin-msteams/assets/100013900/9f0f4f29-f9a2-433b-b90c-444e292ca221)

