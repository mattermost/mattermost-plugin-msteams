import {test, expect } from '@playwright/test';

import MattermostContainer from '../mmcontainer';

let mattermost: MattermostContainer;

test.beforeAll(async () => {
  mattermost = await new MattermostContainer().start();
})

test.afterAll(async () => {
  await mattermost.stop();
})

test('sample testcontainer test', async ({ page }) => {
  const url = mattermost.url()

  await page.goto(url);

  // Click the get started link.
  await page.getByText('View in Browser').click();

  // Expects page to have a heading with the name of Installation.
  await expect(page.getByText('Log in to your account')).toBeVisible();
});
