package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gosimple/slug"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/cluster"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/handlers"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	botUsername           = "msteams"
	botDisplayName        = "MS Teams"
	pluginID              = "com.mattermost.msteams-sync"
	clusterMutexKey       = "subscriptions_cluster_mutex"
	lastReceivedChangeKey = "last_received_change"
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

	stopSubscriptions func()
	stopContext       context.Context

	userID string

	store        store.Store
	clusterMutex *cluster.Mutex

	activityHandler *handlers.ActivityHandler

	clientBuilderWithToken func(string, string, *oauth2.Token, func(string, ...any)) msteams.Client
}

func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	api := NewAPI(p, p.store)
	api.ServeHTTP(w, r)
}

func (p *Plugin) GetAPI() plugin.API {
	return p.API
}

func (p *Plugin) GetStore() store.Store {
	return p.store
}

func (p *Plugin) GetSyncDirectMessages() bool {
	return p.configuration.SyncDirectMessages
}

func (p *Plugin) GetBotUserID() string {
	return p.userID
}

func (p *Plugin) GetClientForApp() msteams.Client {
	return p.msteamsAppClient
}

func (p *Plugin) GetURL() string {
	config := p.API.GetConfig()
	if strings.HasSuffix(*config.ServiceSettings.SiteURL, "/") {
		return *config.ServiceSettings.SiteURL + "plugins/" + pluginID
	}
	return *config.ServiceSettings.SiteURL + "/plugins/" + pluginID
}

func (p *Plugin) GetClientForUser(userID string) (msteams.Client, error) {
	token, _ := p.store.GetTokenForMattermostUser(userID)
	if token == nil {
		return nil, errors.New("not connected user")
	}
	return p.clientBuilderWithToken(p.configuration.TenantID, p.configuration.ClientID, token, p.API.LogError), nil
}

func (p *Plugin) GetClientForTeamsUser(teamsUserID string) (msteams.Client, error) {
	token, _ := p.store.GetTokenForMSTeamsUser(teamsUserID)
	if token == nil {
		return nil, errors.New("not connected user")
	}

	return p.clientBuilderWithToken(p.configuration.TenantID, p.configuration.ClientID, token, p.API.LogError), nil
}

func (p *Plugin) connectTeamsAppClient() error {
	p.msteamsAppClientMutex.Lock()
	defer p.msteamsAppClientMutex.Unlock()

	if p.msteamsAppClient == nil {
		p.msteamsAppClient = msteams.NewApp(
			p.configuration.TenantID,
			p.configuration.ClientID,
			p.configuration.ClientSecret,
			p.API.LogError,
		)
	}
	err := p.msteamsAppClient.Connect()
	if err != nil {
		p.API.LogError("Unable to connect to the app client", "error", err)
		return err
	}
	return nil
}

func (p *Plugin) start(syncSince *time.Time) {
	p.activityHandler.Start()

	err := p.connectTeamsAppClient()
	if err != nil {
		p.API.LogError("Unable to connect to the msteams", "error", err)
		return
	}

	ctx, stop := context.WithCancel(context.Background())
	p.stopSubscriptions = stop
	p.stopContext = ctx

	if p.configuration.SyncUsers > 0 {
		go p.syncUsersPeriodically(ctx, p.configuration.SyncUsers)
	}

	go p.startSubscriptions()
	if syncSince != nil {
		go p.syncSince(*syncSince)
	}
}

func (p *Plugin) syncSince(syncSince time.Time) {
	// TODO: Implement the sync mechanism
	p.API.LogDebug("Syncing since", "date", syncSince)
}

func (p *Plugin) startSubscriptions() {
	p.clusterMutex.Lock()
	defer p.clusterMutex.Unlock()

	counter := 0
	maxRetries := 20
	for {
		resp, _ := http.Post(p.GetURL()+"/changes?validationToken=test-alive", "text/html", bytes.NewReader([]byte{}))
		if resp != nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}

		counter++
		if counter > maxRetries {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	_, err := p.msteamsAppClient.SubscribeToChannels(p.GetURL()+"/", p.configuration.WebhookSecret, !p.configuration.EvaluationAPI)
	if err != nil {
		p.API.LogError("Unable to subscribe to channels", "error", err)
		return
	}

	_, err = p.msteamsAppClient.SubscribeToChats(p.GetURL()+"/", p.configuration.WebhookSecret, !p.configuration.EvaluationAPI)
	if err != nil {
		p.API.LogError("Unable to subscribe to chats", "error", err)
		return
	}
}

func (p *Plugin) stop() {
	if p.stopSubscriptions != nil {
		p.stopSubscriptions()
		time.Sleep(1 * time.Second)
	}
	if p.activityHandler != nil {
		p.activityHandler.Stop()
	}
}

func (p *Plugin) restart() {
	p.stop()
	p.start(nil)
}

func (p *Plugin) generatePluginSecrets() error {
	needSaveConfig := false
	if p.configuration.WebhookSecret == "" {
		secret, err := generateSecret()
		if err != nil {
			return err
		}

		p.configuration.WebhookSecret = secret
		needSaveConfig = true
	}
	if p.configuration.EncryptionKey == "" {
		secret, err := generateSecret()
		if err != nil {
			return err
		}

		p.configuration.EncryptionKey = secret
		needSaveConfig = true
	}
	if needSaveConfig {
		configMap, err := p.configuration.ToMap()
		if err != nil {
			return err
		}
		if appErr := p.API.SavePluginConfig(configMap); appErr != nil {
			return appErr
		}
	}
	return nil
}

func (p *Plugin) OnActivate() error {
	if p.clientBuilderWithToken == nil {
		p.clientBuilderWithToken = msteams.NewTokenClient
	}
	err := p.generatePluginSecrets()
	if err != nil {
		return err
	}

	data, appErr := p.API.KVGet(lastReceivedChangeKey)
	if appErr != nil {
		return appErr
	}

	lastReceivedChangeMicro := int64(0)
	var lastRecivedChange *time.Time
	if len(data) > 0 {
		lastReceivedChangeMicro, err = strconv.ParseInt(string(data), 10, 64)
		if err != nil {
			return err
		}
		parsedTime := time.UnixMicro(lastReceivedChangeMicro)
		lastRecivedChange = &parsedTime
	}

	client := pluginapi.NewClient(p.API, p.Driver)

	p.activityHandler = handlers.New(p)

	clusterMutex, err := cluster.NewMutex(p.API, clusterMutexKey)
	if err != nil {
		return err
	}
	botID, err := client.Bot.EnsureBot(&model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: "Created by the MS Teams Sync plugin.",
	}, pluginapi.ProfileImagePath("assets/msteams-sync-icon.png"))
	if err != nil {
		return err
	}
	p.userID = botID
	p.clusterMutex = clusterMutex

	err = p.API.RegisterCommand(p.createMsteamsSyncCommand())
	if err != nil {
		return err
	}

	if p.store == nil {
		db, err := client.Store.GetMasterDB()
		if err != nil {
			return err
		}

		p.store = store.New(
			db,
			client.Store.DriverName(),
			p.API,
			func() []string { return strings.Split(p.configuration.EnabledTeams, ",") },
			func() []byte { return []byte(p.configuration.EncryptionKey) },
		)
		if err := p.store.Init(); err != nil {
			return err
		}
	}

	go p.start(lastRecivedChange)
	return nil
}

func (p *Plugin) OnDeactivate() error {
	p.stop()
	return nil
}

func (p *Plugin) syncUsersPeriodically(ctx context.Context, minutes int) {
	p.syncUsers()
	for {
		select {
		case <-time.After(time.Duration(minutes) * time.Minute):
			p.syncUsers()
		case <-ctx.Done():
			return
		}
	}
}

func (p *Plugin) syncUsers() {
	msUsers, err := p.msteamsAppClient.ListUsers()
	if err != nil {
		p.API.LogError("Unable to sync users", "error", err)
		return
	}
	mmUsers, appErr := p.API.GetUsers(&model.UserGetOptions{Active: true, Page: 0, PerPage: math.MaxInt32})
	if appErr != nil {
		p.API.LogError("Unable to sync users", "error", appErr)
		return
	}

	mmUsersMap := make(map[string]*model.User, len(mmUsers))
	for _, u := range mmUsers {
		mmUsersMap[u.Email] = u
	}

	for _, msUser := range msUsers {
		mmUser, ok := mmUsersMap[msUser.ID+"@msteamssync"]

		username := slug.Make(msUser.DisplayName) + "-" + msUser.ID

		if !ok {
			userUUID := uuid.Parse(msUser.ID)
			encoding := base32.NewEncoding("ybndrfg8ejkmcpqxot1uwisza345h769").WithPadding(base32.NoPadding)
			shortUserID := encoding.EncodeToString(userUUID)

			newMMUser := &model.User{
				Password:  model.NewId(),
				Email:     msUser.ID + "@msteamssync",
				RemoteId:  &shortUserID,
				FirstName: msUser.DisplayName,
				Username:  username,
			}

			newUser, appErr := p.API.CreateUser(newMMUser)
			if appErr != nil {
				p.API.LogError("Unable to sync user", "error", appErr)
				continue
			}

			err = p.store.SetUserInfo(newUser.Id, msUser.ID, nil)
			if err != nil {
				p.API.LogError("Unable to sync user", "error", err)
			}
		} else if username != mmUser.Username || msUser.DisplayName != mmUser.FirstName {
			mmUser.Username = username
			mmUser.FirstName = msUser.DisplayName
			_, err := p.API.UpdateUser(mmUser)
			if err != nil {
				p.API.LogError("Unable to sync user", "error", err)
			}
		}
	}
}

func generateSecret() (string, error) {
	b := make([]byte, 256)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	s := base64.RawStdEncoding.EncodeToString(b)
	s = s[:32]
	return s, nil
}
