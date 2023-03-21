# Setting up an OAuth application in Azure

### Step 1: Create Mattermost App in Azure

1. Sign into [portal.azure.com](https://portal.azure.com) using an admin Azure account.
2. Navigate to [App Registrations](https://portal.azure.com/#blade/Microsoft_AAD_IAM/ActiveDirectoryMenuBlade/RegisteredApps)
3. Click on **New registration** at the top of the page.

![image](https://user-images.githubusercontent.com/6913320/76347903-be67f580-62dd-11ea-829e-236dd45865a8.png)

4. Then fill out the form with the following values:

- Name: `Mattermost MS Teams Sync`
- Supported account types: Default value (Single tenant)

Select **Register** to submit the form.

![image](https://user-images.githubusercontent.com/77336594/226331343-18b8341b-603a-4cd1-b2fa-81b7573938e4.png)

5. Navigate to **Authentication** in the left pane.

6. Scroll down to **Advanced settings** and enable the "Allow public client flows" toggle button.

![image](https://user-images.githubusercontent.com/77336594/226343720-83e95945-31b8-4ff6-8de5-4fe90904adaa.png)

7. Navigate to **Certificates & secrets** in the left pane.

8. Click on **New client secret**. Then click on **Add**, and copy the new secret on the bottom right corner of the screen. We'll use this value later in the Mattermost admin console.

![image](https://user-images.githubusercontent.com/77336594/226332268-93b8fa85-ba5b-4fcc-938b-ca8d642b8521.png)

9. Navigate to **API permissions** in the left pane.

10. Click on **Add a permission**, then **Microsoft Graph** in the right pane.

![image](https://user-images.githubusercontent.com/6913320/76350226-c2961200-62e1-11ea-9080-19a9b75c2aee.png)

11. Click on **Delegated permissions**, and scroll down to select the following permissions:

- `ChannelMessage.Send`
- `Chat.Create`
- `Files.ReadWrite`
- `Team.ReadBasic.All`
- `Channel.ReadBasic.All`
- `Chat.ReadWrite`
- `ChatMessage.ReadAll`
- `offline_access`
- `User.Read`

12. Click on **Add permissions** to submit the form.

13. Next, add application permissions via **Add a permission > Microsoft Graph > Application permissions**.

14. Select the following permissions:

- `ChannelMessage.Read.All`
- `Chat.Read.All`
- `User.Read.All`
- `Channel.ReadBasic.All`
- `Team.ReadBasic.All`

15. Click on **Add permissions** to submit the form.

16. Click on **Grant admin consent for...** to grant the permissions for the application.

You're all set for configuration inside of Azure.
