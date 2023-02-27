package main

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost-plugin-api/cluster"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/links"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
)

const (
	botUsername     = "msteams"
	botDisplayName  = "MS Teams"
	pluginID        = "com.mattermost.msteams-sync-plugin"
	clusterMutexKey = "subscriptions_cluster_mutex"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	msteamsAppClientMutex sync.Mutex
	msteamsAppClient      msteams.Client
	msteamsBotClientMutex sync.Mutex
	msteamsBotClient      msteams.Client

	botID  string
	userID string

	links        *links.LinksService
	store        store.Store
	clusterMutex *cluster.Mutex
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	api := NewAPI(p, p.store)
	api.ServeHTTP(w, r)
}

func (p *Plugin) getURL() string {
	config := p.API.GetConfig()
	if strings.HasSuffix(*config.ServiceSettings.SiteURL, "/") {
		return *config.ServiceSettings.SiteURL + "plugins/" + pluginID
	}
	return *config.ServiceSettings.SiteURL + "/plugins/" + pluginID
}

func (p *Plugin) connectTeamsAppClient() error {
	p.msteamsAppClientMutex.Lock()
	defer p.msteamsAppClientMutex.Unlock()

	if p.msteamsAppClient == nil {
		p.msteamsAppClient = msteams.NewApp(
			p.configuration.TenantId,
			p.configuration.ClientId,
			p.configuration.ClientSecret,
		)
	}
	err := p.msteamsAppClient.Connect()
	if err != nil {
		p.API.LogError("Unable to connect to the app client", "error", err)
		return err
	}
	return nil
}

func (p *Plugin) connectTeamsBotClient() error {
	p.msteamsBotClientMutex.Lock()
	defer p.msteamsBotClientMutex.Unlock()
	if p.msteamsBotClient == nil {
		p.msteamsBotClient = msteams.NewBot(
			p.configuration.TenantId,
			p.configuration.ClientId,
			p.configuration.ClientSecret,
			p.configuration.BotUsername,
			p.configuration.BotPassword,
		)
	}
	err := p.msteamsBotClient.Connect()
	if err != nil {
		p.API.LogError("Unable to connect to the bot client", "error", err)
		return err
	}
	return nil
}

func (p *Plugin) start() {
	if p.links != nil {
		if err := p.links.Start(); err != nil {
			p.API.LogError("Unable to start the links service", "error", err)
			p.links = nil
			return
		}
	}
}

func (p *Plugin) stop() {
	if p.links != nil {
		p.links.Stop()
	}
}

func (p *Plugin) restart() {
	p.stop()
	p.start()
}

func (p *Plugin) OnActivate() error {
	clusterMutex, err := cluster.NewMutex(p.API, clusterMutexKey)
	if err != nil {
		return err
	}
	botID, appErr := p.API.EnsureBotUser(&model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: "Created by the MS Teams Sync plugin.",
	})
	if appErr != nil {
		return appErr
	}
	p.userID = botID
	p.clusterMutex = clusterMutex

	appErr = p.API.RegisterCommand(createMsteamsSyncCommand())
	if appErr != nil {
		return appErr
	}

	p.links = links.New(
		p.API,
		func() msteams.Client { return p.msteamsAppClient },
		func() string { return p.configuration.WebhookSecret },
		func() string { return p.getURL() + "/" },
	)

	err = p.connectTeamsAppClient()
	if err != nil {
		p.API.LogError("Unable to connect to the msteams", "error", err)
		return err
	}
	err = p.connectTeamsBotClient()
	if err != nil {
		p.API.LogError("Unable to connect to the msteams", "error", err)
		return err
	}

	lockctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	err = p.clusterMutex.LockWithContext(lockctx)
	if err != nil {
		p.API.LogInfo("Other node is taking care of the subscriptions")
		return nil
	}
	defer p.clusterMutex.Unlock()
	time.Sleep(100 * time.Millisecond)

	p.store = store.New(p.API, func() []string { return strings.Split(p.configuration.EnabledTeams, ",") })

	go p.start()
	return nil
}

func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	if post.Props != nil {
		if _, ok := post.Props["msteams_sync_"+p.userID].(bool); ok {
			return
		}
	}

	link, err := p.store.GetLinkByChannelID(post.ChannelId)
	if err != nil || link == nil {
		channel, err := p.API.GetChannel(post.ChannelId)
		if err != nil {
			return
		}
		if channel.Type == model.ChannelTypeDirect {
			members, err := p.API.GetChannelMembers(post.ChannelId, 0, 2)
			if err != nil {
				return
			}
			var dstUser string
			for _, m := range members {
				if m.UserId != post.UserId {
					dstUser = m.UserId
				}
			}
			p.SendChat(dstUser, post.UserId, post)
		}
		if channel.Type == model.ChannelTypeGroup {
			// TODO: Add support for group messages
			panic("Fix this for group messages")
		}
		return
	}

	user, _ := p.API.GetUser(post.UserId)

	p.Send(link, user, post)
}

func (p *Plugin) MessageHasBeenUpdated(c *plugin.Context, newPost, oldPost *model.Post) {
	if oldPost.Props != nil {
		if _, ok := oldPost.Props["msteams_sync_"+p.userID].(bool); ok {
			return
		}
	}

	link, err := p.store.GetLinkByChannelID(newPost.ChannelId)
	if err != nil || link == nil {
		return
	}

	user, _ := p.API.GetUser(newPost.UserId)

	p.Update(link, user, newPost, oldPost)
}

func (p *Plugin) OnDeactivate() error {
	p.stop()
	return nil
}
func (p *Plugin) checkEnabledTeamByTeamId(teamId string) bool {
	if p.configuration.EnabledTeams == "" {
		return true
	}
	team, appErr := p.API.GetTeam(teamId)
	if appErr != nil {
		return false
	}
	isTeamEnabled := false
	enabledTeams := strings.Split(p.configuration.EnabledTeams, ",")
	for _, enabledTeam := range enabledTeams {
		if team.Name == enabledTeam {
			isTeamEnabled = true
			break
		}
	}
	return isTeamEnabled
}

func (p *Plugin) SendChat(dstUser, srcUser string, post *model.Post) (string, error) {
	p.API.LogDebug("Sending direct message to MS Teams", "srcUser", srcUser, "dstUser", dstUser, "post", post)

	parentID := ""
	if post.RootId != "" {
		parentID, _ = p.store.MattermostToTeamsPostId(post.RootId)
	}

	dstUserID, err := p.store.MattermostToTeamsUserId(dstUser)
	if err != nil {
		return "", err
	}
	srcUserID, err := p.store.MattermostToTeamsUserId(dstUser)
	if err != nil {
		return "", err
	}

	p.API.LogDebug("Sending direct message to MS Teams", "srcUserID", srcUserID, "dstUserID", dstUserID, "post", post)
	token, _ := p.store.GetTokenForMattermostUser(srcUser)
	text := post.Message
	if token == nil {
		return "", errors.New("not connected user")
	}
	client := msteams.NewTokenClient(token)

	chatID, err := client.CreateOrGetChatForUsers(dstUserID, srcUserID)
	if err != nil {
		p.API.LogError("FAILING TO CREATE OR GET THE CHAT", "error", err)
		return "", err
	}

	var attachments []*msteams.Attachment
	// TODO: Fix attachments here later
	// for _, fileId := range post.FileIds {
	// 	fileInfo, appErr := p.API.GetFileInfo(fileId)
	// 	if appErr != nil {
	// 		p.API.LogWarn("Unable to get file attachment", "error", appErr)
	// 		continue
	// 	}
	// 	fileData, appErr := p.API.GetFile(fileInfo.Id)
	// 	if appErr != nil {
	// 		p.API.LogWarn("error get file attachment from mattermost", "error", appErr)
	// 		continue
	// 	}

	// 	attachment, err := client.UploadFile(link.MSTeamsTeam, link.MSTeamsChannel, fileInfo.Id+"_"+fileInfo.Name, int(fileInfo.Size), fileInfo.MimeType, bytes.NewReader(fileData))
	// 	if err != nil {
	// 		p.API.LogWarn("error uploading attachment", "error", err)
	// 		continue
	// 	}
	// 	attachments = append(attachments, attachment)
	// }

	newMessageId, err := client.SendChatWithAttachments(chatID, parentID, text, attachments)
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err)
		return "", err
	}

	if post.Id != "" && newMessageId != "" {
		p.store.LinkPosts(post.Id, newMessageId)
	}
	return newMessageId, nil
}

func (p *Plugin) Send(link *links.ChannelLink, user *model.User, post *model.Post) (string, error) {
	p.API.LogDebug("Sending message to MS Teams", "link", link, "post", post)

	parentID := ""
	if post.RootId != "" {
		parentID, _ = p.store.MattermostToTeamsPostId(post.RootId)
	}

	client := p.msteamsBotClient
	token, _ := p.store.GetTokenForMattermostUser(user.Id)
	text := post.Message
	if token != nil {
		client = msteams.NewTokenClient(token)
	} else {
		text = user.Username + ":\n\n" + post.Message
	}

	var attachments []*msteams.Attachment
	for _, fileId := range post.FileIds {
		fileInfo, appErr := p.API.GetFileInfo(fileId)
		if appErr != nil {
			p.API.LogWarn("Unable to get file attachment", "error", appErr)
			continue
		}
		fileData, appErr := p.API.GetFile(fileInfo.Id)
		if appErr != nil {
			p.API.LogWarn("error get file attachment from mattermost", "error", appErr)
			continue
		}

		attachment, err := client.UploadFile(link.MSTeamsTeam, link.MSTeamsChannel, fileInfo.Id+"_"+fileInfo.Name, int(fileInfo.Size), fileInfo.MimeType, bytes.NewReader(fileData))
		if err != nil {
			p.API.LogWarn("error uploading attachment", "error", err)
			continue
		}
		attachments = append(attachments, attachment)
	}

	newMessageId, err := client.SendMessageWithAttachments(link.MSTeamsTeam, link.MSTeamsChannel, parentID, text, attachments)
	if err != nil {
		p.API.LogWarn("Error creating post", "error", err)
		return "", err
	}

	if post.Id != "" && newMessageId != "" {
		p.store.LinkPosts(post.Id, newMessageId)
	}
	return newMessageId, nil
}

func (p *Plugin) Delete(link links.ChannelLink, user *model.User, post *model.Post) error {
	p.API.LogDebug("Sending message to MS Teams", "link", link, "post", post)

	parentID := ""
	if post.RootId != "" {
		parentID, _ = p.store.MattermostToTeamsPostId(post.RootId)
	}

	msgID, _ := p.store.MattermostToTeamsPostId(post.Id)

	err := p.msteamsBotClient.DeleteMessage(link.MSTeamsTeam, link.MSTeamsChannel, parentID, msgID)
	if err != nil {
		p.API.LogError("Error deleting post", "error", err)
		return err
	}
	return nil
}

func (p *Plugin) Update(link *links.ChannelLink, user *model.User, newPost, oldPost *model.Post) error {
	p.API.LogDebug("Sending message to MS Teams", "link", link, "oldPost", oldPost, "newPost", newPost)

	parentID := ""
	if oldPost.RootId != "" {
		parentID, _ = p.store.MattermostToTeamsPostId(newPost.RootId)
	}

	msgID, _ := p.store.MattermostToTeamsPostId(newPost.Id)

	text := user.Username + ":\n\n " + newPost.Message

	err := p.msteamsBotClient.UpdateMessage(link.MSTeamsTeam, link.MSTeamsChannel, parentID, msgID, text)
	if err != nil {
		p.API.LogWarn("Error updating the post", "error", err)
		return err
	}

	return nil
}
