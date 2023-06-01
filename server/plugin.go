package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base32"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gosimple/slug"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/cluster"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/handlers"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/monitor"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
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
	monitor      *monitor.Monitor

	activityHandler *handlers.ActivityHandler

	clientBuilderWithToken func(string, string, string, string, *oauth2.Token, func(string, ...any)) msteams.Client
	publicKey              *rsa.PublicKey
	privateKey             *rsa.PrivateKey
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
	return p.getConfiguration().SyncDirectMessages
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
	return p.clientBuilderWithToken(p.GetURL()+"/oauth-redirect", p.getConfiguration().TenantID, p.getConfiguration().ClientID, p.getConfiguration().ClientSecret, token, p.API.LogError), nil
}

func (p *Plugin) GetClientForTeamsUser(teamsUserID string) (msteams.Client, error) {
	token, _ := p.store.GetTokenForMSTeamsUser(teamsUserID)
	if token == nil {
		return nil, errors.New("not connected user")
	}

	return p.clientBuilderWithToken(p.GetURL()+"/oauth-redirect", p.getConfiguration().TenantID, p.getConfiguration().ClientID, p.getConfiguration().ClientSecret, token, p.API.LogError), nil
}

func (p *Plugin) connectTeamsAppClient() error {
	p.msteamsAppClientMutex.Lock()
	defer p.msteamsAppClientMutex.Unlock()

	if p.msteamsAppClient == nil {
		p.msteamsAppClient = msteams.NewApp(
			p.getConfiguration().TenantID,
			p.getConfiguration().ClientID,
			p.getConfiguration().ClientSecret,
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

	p.monitor = monitor.New(p.msteamsAppClient, p.store, p.API, p.GetURL()+"/", p.getConfiguration().WebhookSecret, p.getConfiguration().EvaluationAPI, p.getBase64Certificate())
	if err = p.monitor.Start(); err != nil {
		p.API.LogError("Unable to start the monitoring system", "error", err)
	}

	ctx, stop := context.WithCancel(context.Background())
	p.stopSubscriptions = stop
	p.stopContext = ctx

	if p.getConfiguration().SyncUsers > 0 {
		go p.syncUsersPeriodically(ctx, p.getConfiguration().SyncUsers)
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

func (p *Plugin) getBase64Certificate() string {
	certificate := p.getConfiguration().CertificatePublic
	if certificate == "" {
		return ""
	}
	block, _ := pem.Decode([]byte(certificate))
	return base64.StdEncoding.EncodeToString(pem.EncodeToMemory(block))
}

func (p *Plugin) getPrivateKey() (*rsa.PrivateKey, error) {
	keyPemString := p.getConfiguration().CertificateKey
	if keyPemString == "" {
		return nil, errors.New("certificate private key not configured")
	}
	privPem, _ := pem.Decode([]byte(keyPemString))
	var privPemBytes []byte
	if privPem.Type != "RSA PRIVATE KEY" {
		p.API.LogDebug("RSA private key is of the wrong type", "type", privPem.Type)
	}
	privPemBytes = privPem.Bytes

	var err error
	var parsedKey interface{}
	if parsedKey, err = x509.ParsePKCS1PrivateKey(privPemBytes); err != nil {
		if parsedKey, err = x509.ParsePKCS8PrivateKey(privPemBytes); err != nil { // note this returns type `interface{}`
			if parsedKey, err = x509.ParseECPrivateKey(privPemBytes); err != nil {
				return nil, err
			}
		}
	}

	var privateKey *rsa.PrivateKey
	var ok bool
	privateKey, ok = parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("Not valid key")
	}

	// publicPemString := p.getConfiguration().CertificatePublic
	// pubPem, _ := pem.Decode([]byte(publicPemString))
	// if pubPem == nil || pubPem.Type != "RSA PUBLIC KEY" {
	// 	// return nil, errors.New("invalid key type")
	// 	p.API.LogDebug("RSA public key is of the wrong type", "type", privPem.Type)
	// }

	// if parsedKey, err = x509.ParsePKIXPublicKey(pubPem.Bytes); err != nil {
	// 	return nil, err
	// }

	// var pubKey *rsa.PublicKey
	// if pubKey, ok = parsedKey.(*rsa.PublicKey); !ok {
	// 	return nil, errors.New("not valid public certificate")
	// }

	// privateKey.PublicKey = *pubKey

	return privateKey, nil
}

func (p *Plugin) Decrypt(ciphertext []byte) ([]byte, error) {
	key, err := p.getPrivateKey()
	if err != nil {
		p.API.LogDebug("Unable to get private key", "error", err)
		return nil, err
	}
	hash := sha1.New()
	plaintext, err := rsa.DecryptOAEP(hash, rand.Reader, key, ciphertext, nil)
	if err != nil {
		p.API.LogDebug("Unable to decrypt data", "error", err, "cipheredText", string(ciphertext))
		return nil, err
	}
	return plaintext, nil
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

	links, err := p.store.ListChannelLinks()
	if err != nil {
		p.API.LogError("Unable to list channel links", "error", err)
		return
	}

	for _, link := range links {
		channelsSubscription, err2 := p.msteamsAppClient.SubscribeToChannel(link.MSTeamsTeam, link.MSTeamsChannel, p.GetURL()+"/", p.getConfiguration().WebhookSecret, p.getBase64Certificate())
		if err2 != nil {
			p.API.LogError("Unable to subscribe to channels", "error", err2)
			continue
		}

		err2 = p.store.SaveChannelSubscription(storemodels.ChannelSubscription{
			SubscriptionID: channelsSubscription.ID,
			TeamID:         link.MSTeamsTeam,
			ChannelID:      link.MSTeamsChannel,
			ExpiresOn:      channelsSubscription.ExpiresOn,
			Secret:         p.getConfiguration().WebhookSecret,
		})
		if err2 != nil {
			p.API.LogError("Unable to save the channel subscription for monitoring system", "error", err2)
			continue
		}
	}

	chatsSubscription, err := p.msteamsAppClient.SubscribeToChats(p.GetURL()+"/", p.getConfiguration().WebhookSecret, !p.getConfiguration().EvaluationAPI, p.getBase64Certificate())
	if err != nil {
		p.API.LogError("Unable to subscribe to chats", "error", err)
		return
	}

	err = p.store.SaveGlobalSubscription(storemodels.GlobalSubscription{
		SubscriptionID: chatsSubscription.ID,
		Type:           "allChats",
		ExpiresOn:      chatsSubscription.ExpiresOn,
		Secret:         p.getConfiguration().WebhookSecret,
	})
	if err != nil {
		p.API.LogError("Unable to save the chats subscription for monitoring system", "error", err)
		return
	}
}

func (p *Plugin) stop() {
	if p.monitor != nil {
		p.monitor.Stop()
	}
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
	p.API.LogDebug("Starting sync user job")
	msUsers, err := p.msteamsAppClient.ListUsers()
	if err != nil {
		p.API.LogError("Unable to list MS Teams users during sync user job", "error", err.Error())
		return
	}

	p.API.LogDebug("Count of MS Teams users", "count", len(msUsers))
	mmUsers, appErr := p.API.GetUsers(&model.UserGetOptions{Active: true, Page: 0, PerPage: math.MaxInt32})
	if appErr != nil {
		p.API.LogError("Unable to get MM users during sync user job", "error", appErr.Error())
		return
	}

	p.API.LogDebug("Count of MM users", "count", len(mmUsers))
	mmUsersMap := make(map[string]*model.User, len(mmUsers))
	for _, u := range mmUsers {
		mmUsersMap[u.Email] = u
	}

	for _, msUser := range msUsers {
		userSuffixID := 1
		if msUser.Mail == "" {
			continue
		}

		p.API.LogDebug("Running sync user job for user with email", "email", msUser.Mail)

		mmUser, ok := mmUsersMap[msUser.Mail]

		username := "msteams_" + slug.Make(msUser.DisplayName)
		if !ok {
			userUUID := uuid.Parse(msUser.ID)
			encoding := base32.NewEncoding("ybndrfg8ejkmcpqxot1uwisza345h769").WithPadding(base32.NoPadding)
			shortUserID := encoding.EncodeToString(userUUID)

			newMMUser := &model.User{
				Password:  p.GenerateRandomPassword(),
				Email:     msUser.Mail,
				RemoteId:  &shortUserID,
				FirstName: msUser.DisplayName,
				Username:  username,
			}

			var newUser *model.User
			for {
				newUser, appErr = p.API.CreateUser(newMMUser)
				if appErr != nil {
					if appErr.Id == "app.user.save.username_exists.app_error" {
						newMMUser.Username += "-" + fmt.Sprint(userSuffixID)
						userSuffixID++
						continue
					}

					p.API.LogError("Unable to create new MM user during sync job", "email", msUser.Mail, "error", appErr.Error())
					break
				}

				break
			}

			if newUser == nil || newUser.Id == "" {
				continue
			}

			err = p.store.SetUserInfo(newUser.Id, msUser.ID, nil)
			if err != nil {
				p.API.LogError("Unable to set user info during sync user job", "email", msUser.Mail, "error", err.Error())
			}
		} else if (username != mmUser.Username || msUser.DisplayName != mmUser.FirstName) && mmUser.RemoteId != nil {
			mmUser.Username = username
			mmUser.FirstName = msUser.DisplayName
			for {
				_, err := p.API.UpdateUser(mmUser)
				if err != nil {
					if err.Id == "app.user.save.username_exists.app_error" {
						mmUser.Username += "-" + fmt.Sprint(userSuffixID)
						userSuffixID++
						continue
					}

					p.API.LogError("Unable to update user during sync user job", "email", mmUser.Email, "error", err.Error())
					break
				}

				break
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

func (p *Plugin) GenerateRandomPassword() string {
	lowerCharSet := "abcdedfghijklmnopqrst"
	upperCharSet := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	specialCharSet := "!@#$%&*"
	numberSet := "0123456789"
	allCharSet := lowerCharSet + upperCharSet + specialCharSet + numberSet

	var password strings.Builder

	password.WriteString(getRandomString(lowerCharSet, 1))
	password.WriteString(getRandomString(upperCharSet, 1))
	password.WriteString(getRandomString(specialCharSet, 1))
	password.WriteString(getRandomString(numberSet, 1))
	password.WriteString(getRandomString(allCharSet, 20))
	return password.String()
}

func getRandomString(characterSet string, length int) string {
	var randomString strings.Builder
	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(length)))
		randomString.WriteString(string(characterSet[num.Int64()]))
	}

	return randomString.String()
}
