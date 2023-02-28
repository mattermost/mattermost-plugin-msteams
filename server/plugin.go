package main

import (
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

func (p *Plugin) getClientForUser(userID string) (msteams.Client, error) {
	token, _ := p.store.GetTokenForMattermostUser(userID)
	if token == nil {
		return nil, errors.New("not connected user")
	}
	return msteams.NewTokenClient(p.configuration.TenantId, p.configuration.ClientId, token), nil
}

func (p *Plugin) getClientForTeamsUser(teamsUserID string) (msteams.Client, error) {
	userID, err := p.store.TeamsToMattermostUserId(teamsUserID)
	if err != nil {
		return nil, err
	}
	if userID == "" {
		return nil, errors.New("not connected user")
	}

	token, _ := p.store.GetTokenForMattermostUser(userID)
	if token == nil {
		return nil, errors.New("not connected user")
	}

	return msteams.NewTokenClient(p.configuration.TenantId, p.configuration.ClientId, token), nil
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

func (p *Plugin) OnDeactivate() error {
	p.stop()
	return nil
}
