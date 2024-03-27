import {test, expect } from '@playwright/test';
import fs from 'fs';

import MattermostContainer from './mmcontainer';

const RunContainer = async (): Promise<MattermostContainer> => {
  let filename = "";
  fs.readdirSync("../dist/").forEach(file => {
      if (file.endsWith(".tar.gz")) {
          filename = "../dist/"+file
      }
  })
  if (filename === "") {
      throw("No tar.gz file found in dist folder")
  }
  const pluginConfig = {
    "clientid":                   "client-id",
  	"clientsecret":               "client-secret",
  	"connectedusersallowed":      1000,
  	"encryptionkey":              "eyPBz0mBhwfGGwce9hp4TWaYzgY7MdIB",
  	"maxSizeForCompleteDownload": 20,
  	"maxsizeforcompletedownload": 20,
  	"tenantid":                   "tenant-id",
  	"webhooksecret":              "webhook-secret",
  }
  const mattermost = await new MattermostContainer()
        .withPlugin(filename, "com.mattermost.msteams-sync", pluginConfig)
        .withEnv("MM_MSTEAMSSYNC_MOCK_CLIENT", "true")
        .start();
  await mattermost.createUser("regularuser@sample.com", "regularuser", "regularuser");
  await mattermost.addUserToTeam("regularuser", "test");
  const userClient = await mattermost.getClient("regularuser", "regularuser")
  const user = await userClient.getMe()
  await userClient.savePreferences(user.id,[
    {user_id: user.id, category: 'tutorial_step', name: user.id, value: '999'},
    {user_id: user.id, category: 'onboarding_task_list', name: 'onboarding_task_list_show', value: 'false'},
    {user_id: user.id, category: 'onboarding_task_list', name: 'onboarding_task_list_open', value: 'false'},
    {
        user_id: user.id,
        category: 'drafts',
        name: 'drafts_tour_tip_showed',
        value: JSON.stringify({drafts_tour_tip_showed: true}),
    },
    {user_id: user.id, category: 'crt_thread_pane_step', name: user.id, value: '999'},
  ]);

  const adminClient = await mattermost.getAdminClient()
  const admin = await adminClient.getMe()
  await adminClient.savePreferences(admin.id,[
    {user_id: admin.id, category: 'tutorial_step', name: admin.id, value: '999'},
    {user_id: admin.id, category: 'onboarding_task_list', name: 'onboarding_task_list_show', value: 'false'},
    {user_id: admin.id, category: 'onboarding_task_list', name: 'onboarding_task_list_open', value: 'false'},
    {
        user_id: admin.id,
        category: 'drafts',
        name: 'drafts_tour_tip_showed',
        value: JSON.stringify({drafts_tour_tip_showed: true}),
    },
    {user_id: admin.id, category: 'crt_thread_pane_step', name: admin.id, value: '999'},
  ]);
  await adminClient.completeSetup({
      organization: "test",
      install_plugins: [],
  });

  return mattermost;
}

export default RunContainer
