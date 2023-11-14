package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

var msClient msteams.Client
var mmClient *model.Client4
var msConnectedTeamId string
var msConnectedChannelId string
var mmConnectedChannelId string

func setup(t *testing.T) {
	if msClient == nil {
		msClient = msteams.NewManualClient(os.Getenv("MSTEAMS_TEST_TENANT_ID"), os.Getenv("MSTEAMS_TEST_CLIENT_ID"), nil)
	}
	if mmClient == nil {
		mmClient = model.NewAPIv4Client(os.Getenv("MATTERMOST_TEST_URL"))
		mmClient.Login(context.Background(), os.Getenv("MATTERMOST_TEST_USERNAME"), os.Getenv("MATTERMOST_TEST_PASSWORD"))
	}
	msConnectedTeamId = os.Getenv("MSTEAMS_TEST_CONNECTED_CHANNEL_TEAM_ID")
	msConnectedChannelId = os.Getenv("MSTEAMS_TEST_CONNECTED_CHANNEL_CHANNEL_ID")
	mmConnectedChannelId = os.Getenv("MATTERMOST_TEST_CONNECTED_CHANNEL_CHANNEL_ID")
}

func TestSendMessageToMSTeamsLinkedChannel(t *testing.T) {
	setup(t)
	generatedMessage := uuid.New().String()
	_, err := msClient.SendMessage(msConnectedTeamId, msConnectedChannelId, "", generatedMessage)
	require.NoError(t, err)
	time.Sleep(3 * time.Second)
	posts, _, err := mmClient.GetPostsForChannel(context.Background(), mmConnectedChannelId, 0, 10, "", false, false)
	require.NoError(t, err)
	for _, post := range posts.Posts {
		if post.Message == generatedMessage {
			return
		}
	}
	t.Fatal("Message sent to MSTeams is not propagated")
}
