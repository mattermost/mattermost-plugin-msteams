//go:build e2e
// +build e2e

package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost/server/public/model"
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
		msClient = msteams.NewManualClient(testCfg.MSTeams.TenantID, testCfg.MSTeams.ClientID, nil)
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
