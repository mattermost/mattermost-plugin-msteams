# Renewing an OAuth application in Azure

### Step 1: Create a new client secret

1. Sign into [portal.azure.com](https://portal.azure.com) using an admin Azure account.
2. Navigate to [App Registrations](https://portal.azure.com/#blade/Microsoft_AAD_IAM/ActiveDirectoryMenuBlade/RegisteredApps)
3. Click on the application you previously created.

![image](./renewal/choose_application.png)

4. Navigate to **Certificates & secrets** in the left pane.

![image](./renewal/choose_certificates_and_secrets.png)

5. Click on **New client secret**. Enter the description and click on **Add**. 

![image](https://user-images.githubusercontent.com/77336594/226332268-93b8fa85-ba5b-4fcc-938b-ca8d642b8521.png)

6. After the creation of the client secret, copy the new secret value, not the secret ID. We'll use this value later in the Mattermost admin console.

![image](./renewal/copy_secret.png)

### Step 2: Configure the MS Teams Plugin

1. Go to the MS Teams plugin configuration page on Mattermost as System Console > Plugins > MSTeams Sync.

![image](./renewal/browse_to_plugin.png)

2. Update the **Client Secret**. DO NOT update the **Client ID**.

![image](./renewal/update_client_secret.png)

3. Save the plugin settings

![image](./renewal/save_plugin_settings.png)

### Step 3. Restart the MS Teams Plugin

1. Go to the **Plugin Management** page on Mattermost as System Console > Plugin Management.

![image](./renewal/plugin_management.png)

2. Scroll to the **MS Teams Sync** plugin and click **Disable**

![image](./renewal/disable_plugin.png)

3. After the plugin has disabled, click **Enable**

![image](./renewal/enable_plugin.png)


