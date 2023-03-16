package main

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/enescakir/emoji"
	"github.com/gosimple/slug"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/cluster"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
)

const (
	botUsername     = "msteams"
	botDisplayName  = "MS Teams"
	pluginID        = "com.mattermost.msteams-sync"
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

	stopSubscriptions func()
	stopContext       context.Context
	startMutex        sync.Mutex

	userID string

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
	return msteams.NewTokenClient(p.configuration.TenantId, p.configuration.ClientId, token, p.API.LogError), nil
}

func (p *Plugin) getClientForTeamsUser(teamsUserID string) (msteams.Client, error) {
	token, _ := p.store.GetTokenForMSTeamsUser(teamsUserID)
	if token == nil {
		return nil, errors.New("not connected user")
	}

	return msteams.NewTokenClient(p.configuration.TenantId, p.configuration.ClientId, token, p.API.LogError), nil
}

func (p *Plugin) connectTeamsAppClient() error {
	p.msteamsAppClientMutex.Lock()
	defer p.msteamsAppClientMutex.Unlock()

	if p.msteamsAppClient == nil {
		p.msteamsAppClient = msteams.NewApp(
			p.configuration.TenantId,
			p.configuration.ClientId,
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
			p.API.LogError,
		)
	}
	err := p.msteamsBotClient.Connect()
	if err != nil {
		p.API.LogError("Unable to connect to the bot client", "error", err)
		return err
	}
	return nil
}

func (p *Plugin) start() error {
	err := p.connectTeamsAppClient()
	if err != nil {
		p.API.LogError("Unable to connect to the msteams", "error", err)
		return err
	}
	err = p.connectTeamsBotClient()
	if err != nil {
		p.API.LogError("Unable to connect to the msteams", "error", err)
		return err
	}

	ctx, stop := context.WithCancel(context.Background())
	p.stopSubscriptions = stop
	p.stopContext = ctx

	if p.configuration.SyncUsers > 0 {
		go p.syncUsersPeriodically(ctx, p.configuration.SyncUsers)
	}

	go p.startSubscriptions(ctx)

	return nil
}

func (p *Plugin) startSubscriptions(ctx context.Context) {
	p.clusterMutex.Lock()
	defer p.clusterMutex.Unlock()

	time.Sleep(100 * time.Millisecond)
	subscriptionID, err := p.msteamsAppClient.SubscribeToChannels(p.getURL()+"/", p.configuration.WebhookSecret)
	if err != nil {
		p.API.LogError("Unable to subscribe to channels", "error", err)
		return
	}

	chatsSubscriptionID, err := p.msteamsAppClient.SubscribeToChats(p.getURL()+"/", p.configuration.WebhookSecret)
	if err != nil {
		p.API.LogError("Unable to subscribe to chats", "error", err)
		return
	}

	go p.msteamsAppClient.RefreshChannelsSubscriptionPeriodically(ctx, p.getURL()+"/", p.configuration.WebhookSecret, subscriptionID)
	go p.msteamsAppClient.RefreshChatsSubscriptionPeriodically(ctx, p.getURL()+"/", p.configuration.WebhookSecret, chatsSubscriptionID)
}

func (p *Plugin) stop() {
	if p.stopSubscriptions != nil {
		p.stopSubscriptions()
		time.Sleep(1 * time.Second)
	}
}

func (p *Plugin) restart() {
	p.stop()
	p.start()
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
	p.generatePluginSecrets()

	client := pluginapi.NewClient(p.API, p.Driver)

	// Initialize the emoji translator
	emojisReverseMap = map[string]string{}
	for alias, unicode := range emoji.Map() {
		emojisReverseMap[unicode] = strings.Replace(alias, ":", "", 2)
	}
	emojisReverseMap["like"] = "+1"
	emojisReverseMap["sad"] = "cry"
	emojisReverseMap["angry"] = "angry"
	emojisReverseMap["laugh"] = "laughing"
	emojisReverseMap["heart"] = "heart"
	emojisReverseMap["surprised"] = "open_mouth"

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

	go p.start()
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
	for _, mmUser := range mmUsers {
		mmUsersMap[mmUser.Email] = mmUser
	}

	for _, msUser := range msUsers {
		mmUser, ok := mmUsersMap[msUser.ID+"@msteamssync"]

		username := slug.Make(msUser.DisplayName) + "-" + msUser.ID

		if !ok {
			userUUID := uuid.Parse(msUser.ID)
			encoding := base32.NewEncoding("ybndrfg8ejkmcpqxot1uwisza345h769").WithPadding(base32.NoPadding)
			shortUserId := encoding.EncodeToString(userUUID)

			mmUser := &model.User{
				Password:  model.NewId(),
				Email:     msUser.ID + "@msteamssync",
				RemoteId:  &shortUserId,
				FirstName: msUser.DisplayName,
				Username:  username,
			}

			newUser, err := p.API.CreateUser(mmUser)
			if err != nil {
				p.API.LogError("Unable to sync user", "error", err)
			}
			p.store.SetUserInfo(newUser.Id, msUser.ID, nil)
		} else {
			if username != mmUser.Username || msUser.DisplayName != mmUser.FirstName {
				mmUser.Username = username
				mmUser.FirstName = msUser.DisplayName
				_, err := p.API.UpdateUser(mmUser)
				if err != nil {
					p.API.LogError("Unable to sync user", "error", err)
				}
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
