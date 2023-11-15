package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
)

type TestConfig struct {
	MSTeams struct {
		TenantId               string `toml:"tenant_id"`
		ClientId               string `toml:"client_id"`
		ConnectedChannelTeamId string `toml:"connected_channel_team_id"`
		ConnectedChannelId     string `toml:"connected_channel_id"`
	} `toml:"msteams"`
	Mattermost struct {
		URL                string `toml:"url"`
		UserUsername       string `toml:"user_username"`
		UserPassword       string `toml:"user_password"`
		AdminUsername      string `toml:"admin_username"`
		AdminPassword      string `toml:"admin_password"`
		ConnectedChannelId string `toml:"connected_channel_id"`
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
		msClient = msteams.NewManualClient(testCfg.MSTeams.TenantId, testCfg.MSTeams.ClientId, nil)
	}
	if mmClient == nil {
		mmClient = model.NewAPIv4Client(testCfg.Mattermost.URL)
		mmClient.Login(context.Background(), testCfg.Mattermost.UserUsername, testCfg.Mattermost.UserPassword)
	}
	if mmClientAdmin == nil {
		mmClientAdmin = model.NewAPIv4Client(testCfg.Mattermost.URL)
		mmClientAdmin.Login(context.Background(), testCfg.Mattermost.AdminUsername, testCfg.Mattermost.AdminPassword)
	}
}

func TestSendMessageAndReplyToMSTeamsLinkedChannel(t *testing.T) {
	setup(t)
	generatedMessage := uuid.New().String()
	generatedReply := uuid.New().String()
	newMessage, err := msClient.SendMessage(testCfg.MSTeams.ConnectedChannelTeamId, testCfg.MSTeams.ConnectedChannelId, "", generatedMessage)
	require.NoError(t, err)

	_, err = msClient.SendMessage(testCfg.MSTeams.ConnectedChannelTeamId, testCfg.MSTeams.ConnectedChannelId, newMessage.ID, generatedReply)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)
	posts, _, err := mmClient.GetPostsForChannel(context.Background(), testCfg.Mattermost.ConnectedChannelId, 0, 10, "", false, false)
	require.NoError(t, err)

	var mattermostNewMessage *model.Post
	var mattermostNewReply *model.Post
	for _, post := range posts.Posts {
		if post.Message == generatedMessage {
			mattermostNewMessage = post
		}
		if post.Message == generatedReply {
			mattermostNewReply = post
		}
	}
	require.NotNil(t, mattermostNewMessage)
	require.NotNil(t, mattermostNewReply)
	require.Equal(t, mattermostNewReply.RootId, mattermostNewMessage.Id)
}

func TestStopPluginSendMessageAndReplyToMSTeamsLinkedChannelStartPlugin(t *testing.T) {
	setup(t)

	t.Log("Disabling the plugin")
	_, err := mmClientAdmin.DisablePlugin(context.Background(), "com.mattermost.msteams-sync")
	if err != nil {
		t.Fatal("Unable to disable the plugin", err)
	}
	defer mmClientAdmin.EnablePlugin(context.Background(), "com.mattermost.msteams-sync")

	generatedMessage := uuid.New().String()
	generatedReply := uuid.New().String()

	t.Log("Sending messages to MSTeams")
	newMessage, err := msClient.SendMessage(testCfg.MSTeams.ConnectedChannelTeamId, testCfg.MSTeams.ConnectedChannelId, "", generatedMessage)
	require.NoError(t, err)

	_, err = msClient.SendMessage(testCfg.MSTeams.ConnectedChannelTeamId, testCfg.MSTeams.ConnectedChannelId, newMessage.ID, generatedReply)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)

	t.Log("Verifying that the messages hasn't been synced to Mattermost")
	posts, _, err := mmClient.GetPostsForChannel(context.Background(), testCfg.Mattermost.ConnectedChannelId, 0, 10, "", false, false)
	require.NoError(t, err)

	var mattermostNewMessage *model.Post
	var mattermostNewReply *model.Post
	for _, post := range posts.Posts {
		if post.Message == generatedMessage {
			mattermostNewMessage = post
		}
		if post.Message == generatedReply {
			mattermostNewReply = post
		}
	}

	require.Nil(t, mattermostNewMessage)
	require.Nil(t, mattermostNewReply)

	t.Log("Enabling the plugin")
	_, err = mmClientAdmin.EnablePlugin(context.Background(), "com.mattermost.msteams-sync")
	if err != nil {
		t.Fatal("Unable to re-enable the plugin", err)
	}

	t.Log("Waiting for the plugin to and sync")
	time.Sleep(5 * time.Second)

	posts, _, err = mmClient.GetPostsForChannel(context.Background(), testCfg.Mattermost.ConnectedChannelId, 0, 10, "", false, false)
	require.NoError(t, err)

	for _, post := range posts.Posts {
		if post.Message == generatedMessage {
			mattermostNewMessage = post
		}
		if post.Message == generatedReply {
			mattermostNewReply = post
		}
	}

	require.NotNil(t, mattermostNewMessage)
	require.NotNil(t, mattermostNewReply)
	require.Equal(t, mattermostNewReply.RootId, mattermostNewMessage.Id)
}

func TestSendMessageAndReplyToMattermostLinkedChannel(t *testing.T) {
	setup(t)
	startTime := time.Now()
	generatedMessage := uuid.New().String()
	me, _, err := mmClient.GetMe(context.Background(), "")
	require.NoError(t, err)
	post := &model.Post{
		Message:   generatedMessage,
		ChannelId: testCfg.Mattermost.ConnectedChannelId,
		UserId:    me.Id,
	}
	newPost, _, err := mmClient.CreatePost(context.Background(), post)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	generatedReply := uuid.New().String()
	replyPost := &model.Post{
		Message:   generatedReply,
		ChannelId: testCfg.Mattermost.ConnectedChannelId,
		UserId:    me.Id,
		RootId:    newPost.Id,
	}
	_, _, err = mmClient.CreatePost(context.Background(), replyPost)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	msTeamsMessages, err := msClient.ListChannelMessages(testCfg.MSTeams.ConnectedChannelTeamId, testCfg.MSTeams.ConnectedChannelId, startTime)
	require.NoError(t, err)

	var msteamsNewMessage *clientmodels.Message
	var msteamsNewReply *clientmodels.Message
	for _, msg := range msTeamsMessages {
		if strings.Contains(msg.Text, generatedMessage) {
			msteamsNewMessage = msg
		}
		if strings.Contains(msg.Text, generatedReply) {
			msteamsNewReply = msg
		}
	}

	require.NotNil(t, msteamsNewMessage)
	require.NotNil(t, msteamsNewReply)
	require.Equal(t, msteamsNewReply.ReplyToID, msteamsNewMessage.ID)
}
