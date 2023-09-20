# Setting up an OAuth application in Azure

### Step 1: Create Mattermost App in Azure

1. Sign into [portal.azure.com](https://portal.azure.com) using an admin Azure account.
2. Navigate to [App Registrations](https://portal.azure.com/#blade/Microsoft_AAD_IAM/ActiveDirectoryMenuBlade/RegisteredApps)
3. Click on **New registration** at the top of the page.

![image](https://user-images.githubusercontent.com/6913320/76347903-be67f580-62dd-11ea-829e-236dd45865a8.png)

4. Fill out the form with the following values:

- Name: `Mattermost MS Teams Sync`
- Supported account types: Default value (Single tenant)
- Platform: Web
- Redirect URI: `https://(MM_SITE_URL)/plugins/com.mattermost.msteams-sync/oauth-redirect`

Replace `(MM_SITE_URL)` with your Mattermost server's Site URL. Select **Register** to submit the form.
Select **Register** to submit the form.

![image](https://github.com/mattermost/mattermost-plugin-msteams-sync/assets/77336594/8a9add4e-307f-4e27-a308-a86a87ebb8e0)

5. Navigate to **Certificates & secrets** in the left pane.

6. Click on **New client secret**. Enter the description and click on **Add**. After the creation of the client secret, copy the new secret value, not the secret ID. We'll use this value later in the Mattermost admin console.

![image](https://user-images.githubusercontent.com/77336594/226332268-93b8fa85-ba5b-4fcc-938b-ca8d642b8521.png)

7. Navigate to **API permissions** in the left pane.

8. Click on **Add a permission**, then **Microsoft Graph** in the right pane.

![image](https://user-images.githubusercontent.com/6913320/76350226-c2961200-62e1-11ea-9080-19a9b75c2aee.png)

9. Click on **Delegated permissions**, and scroll down to select the following permissions:

- `Channel.ReadBasic.All`
- `ChannelMessage.Read.All`
- `ChannelMessage.ReadWrite`
- `ChannelMessage.Send`
- `Chat.Create`
- `Chat.ReadWrite`
- `ChatMessage.Read`
- `Directory.Read.All`
- `Files.Read.All`
- `Files.ReadWrite.All`
- `offline_access`
- `Team.ReadBasic.All`
- `User.Read`

10. Click on **Add permissions** to submit the form.

11. Next, add application permissions via **Add a permission > Microsoft Graph > Application permissions**.

12. Select the following permissions:

- `Channel.ReadBasic.All`
- `ChannelMessage.Read.All`
- `Chat.Read.All`
- `Files.Read.All`
- `Group.Read.All`
- `Team.ReadBasic.All`
- `User.Read.All`

13. Click on **Add permissions** to submit the form.

14. Click on **Grant admin consent for...** to grant the permissions for the application.

### Step 2: Create a user account to act as a bot

1. Create a regular user account. We will connect this account later from the Mattermost side.
1. This account is needed for creating messages on MS Teams on behalf of users who are present in Mattermost but not on MS Teams.
1. This account is also needed when users on Mattermost have not connected their accounts and some messages need to be posted on their behalf. See the screenshot below:

![image](https://user-images.githubusercontent.com/100013900/232403027-6d3ce866-d404-4ef2-a27b-ef5cc897cb25.png)

**Note:** After you've connected the bot user to the dummy account on MS Teams, all the messages that are posted from the dummy account on MS Teams will not be synced back to Mattermost as we consider the dummy account a "bot" and messages from bots are ignored.

### Step 3: Ensure you have the metered APIs enabled (and the pay subscription associated to it)

1. Follow the steps here: https://learn.microsoft.com/en-us/graph/metered-api-setup

1. If you don't configure the metered APIs, you need to use the Evaluation model (configurable in Mattermost) that is limited to a low rate of changes per month. Please, do not use that configuration in real environments because you can stop receiving messages due that limit. See [this doc](https://learn.microsoft.com/en-us/graph/teams-licenses) for more details.

You're all set for configuration inside Azure.
