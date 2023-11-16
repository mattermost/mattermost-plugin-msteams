//go:build e2e
// +build e2e

package main

import (
	"context"
	"os"
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
)

type TestConfig struct {
	MSTeams struct {
		TenantID               string `toml:"tenant_id"`
		ClientID               string `toml:"client_id"`
		ConnectedChannelTeamID string `toml:"connected_channel_team_id"`
		ConnectedChannelID     string `toml:"connected_channel_id"`
		ChatID                 string `toml:"chat_id"`
	} `toml:"msteams"`
	Mattermost struct {
		URL                string `toml:"url"`
		UserUsername       string `toml:"user_username"`
		UserPassword       string `toml:"user_password"`
		AdminUsername      string `toml:"admin_username"`
		AdminPassword      string `toml:"admin_password"`
		ConnectedChannelID string `toml:"connected_channel_id"`
		DmID               string `toml:"dm_id"`
	} `toml:"mattermost"`
}

var msClient msteams.Client
var mmClient *model.Client4
var mmClientAdmin *model.Client4
var testCfg *TestConfig

func setup(t *testing.T) {
	if testCfg == nil {
		data, err := os.ReadFile("testconfig.toml")
		if err != nil {
			t.Fatal(err)
		}
		testCfg = &TestConfig{}
		if err := toml.Unmarshal(data, testCfg); err != nil {
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
