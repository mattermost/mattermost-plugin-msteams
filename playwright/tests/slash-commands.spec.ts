import {test, expect} from '@playwright/test';

import RunContainer from '../plugincontainer';
import MattermostContainer from '../mmcontainer';
import {login, logout} from '../utils';

let mattermost: MattermostContainer;

test.beforeAll(async () => {
  mattermost = await RunContainer()
})

test.afterAll(async () => {
  await mattermost.stop();
})

test.describe('link slash command', () => {
    test('try to link a channel as regular user', async ({ page }) => {
      const url = mattermost.url()
      await login(page, url, "regularuser", "regularuser")
      await expect(page.getByLabel('town square public channel')).toBeVisible();
      await page.getByTestId('post_textbox').fill("/msteams-sync link")
      await page.getByTestId('SendMessageButton').click();

      await expect(page.getByText('Unable to link the channel. You have to be a channel admin to link it.', { exact: true })).toBeVisible();
      await logout(page)
    });

    test('try to link a channel as admin user', async ({ page }) => {
      const url = mattermost.url()
      await login(page, url, "admin", "admin")
      await expect(page.getByLabel('town square public channel')).toBeVisible();
      await page.getByTestId('post_textbox').fill("/msteams-sync link")
      await page.getByTestId('SendMessageButton').click();

      await expect(page.getByText('Invalid link command, please pass the MS Teams team id and channel id as parameters.', { exact: true })).toBeVisible();

      await page.getByTestId('post_textbox').fill("/msteams-sync link team-id channel-id")
      await page.getByTestId('SendMessageButton').click();

      await expect(page.getByText('Unable to link the channel, looks like your account is not connected to MS Teams', { exact: true })).toBeVisible();

      await logout(page);

      // TODO: Implement the cases where the user is correctly connected to MS Teams (We need the mock client to be able to do that)
    });
});
