// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

//go:build e2e
// +build e2e

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost/server/public/model"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/stretchr/testify/require"
)

type TestConfig struct {
	MSTeams struct {
		TenantID               string `json:"tenant_id"`
		ClientID               string `json:"client_id"`
		ConnectedChannelTeamID string `json:"connected_channel_team_id"`
		ConnectedChannelID     string `json:"connected_channel_id"`
		ChatID                 string `json:"chat_id"`
	} `json:"msteams"`
	Mattermost struct {
		URL                string `json:"url"`
		UserUsername       string `json:"user_username"`
		UserPassword       string `json:"user_password"`
		AdminUsername      string `json:"admin_username"`
		AdminPassword      string `json:"admin_password"`
		ConnectedChannelID string `json:"connected_channel_id"`
		DmID               string `json:"dm_id"`
	} `json:"mattermost"`
}

var msClient msteams.Client
var mmClient *model.Client4
var mmClientAdmin *model.Client4
var testCfg *TestConfig

func newManualClient(tenantID, clientID string, logService *pluginapi.LogService) msteams.Client {
	cred, err := azidentity.NewDeviceCodeCredential(&azidentity.DeviceCodeCredentialOptions{
		TenantID: tenantID,
		ClientID: clientID,
		UserPrompt: func(ctx context.Context, message azidentity.DeviceCodeMessage) error {
			fmt.Println(message.Message)
			return nil
		},
	})
	if err != nil {
		fmt.Printf("Error creating credentials: %v\n", err)
		return nil
	}

	client, err := msgraphsdk.NewGraphServiceClientWithCredentials(cred, msteams.TeamsDefaultScopes)
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return nil
	}
	return msteams.NewManualClient(tenantID, clientID, logService, client)
}

func setup(t *testing.T) {
	if testCfg == nil {
		data, err := os.ReadFile("testconfig.json")
		if err != nil {
			t.Fatal("testconfig.json file not found, please read the `server/e2e/README.md` file for more information")
		}
		testCfg = &TestConfig{}
		if err := json.Unmarshal(data, testCfg); err != nil {
			t.Fatal(err)
		}
	}

	if msClient == nil {
		msClient = newManualClient(testCfg.MSTeams.TenantID, testCfg.MSTeams.ClientID, nil)
	}
	if mmClient == nil {
		mmClient = model.NewAPIv4Client(testCfg.Mattermost.URL)
		_, _, err := mmClient.Login(context.Background(), testCfg.Mattermost.UserUsername, testCfg.Mattermost.UserPassword)
		require.NoError(t, err)
	}
	if mmClientAdmin == nil {
		mmClientAdmin = model.NewAPIv4Client(testCfg.Mattermost.URL)
		_, _, err := mmClientAdmin.Login(context.Background(), testCfg.Mattermost.AdminUsername, testCfg.Mattermost.AdminPassword)
		require.NoError(t, err)
	}
}
