package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/gosimple/slug"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-msteams/assets"
	"github.com/mattermost/mattermost-plugin-msteams/server/handlers"
	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/monitor"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/client_disconnectionlayer"
	client_timerlayer "github.com/mattermost/mattermost-plugin-msteams/server/msteams/client_timerlayer"
	"github.com/mattermost/mattermost-plugin-msteams/server/store"
	sqlstore "github.com/mattermost/mattermost-plugin-msteams/server/store/sqlstore"
	timerlayer "github.com/mattermost/mattermost-plugin-msteams/server/store/timerlayer"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
)

const (
	botUsername                  = "msteams"
	botDisplayName               = "MS Teams"
	pluginID                     = "com.mattermost.msteams-sync"
	subscriptionsClusterMutexKey = "subscriptions_cluster_mutex"
	connectClusterMutexKey       = "connect_cluster_mutex"
	msteamsUserTypeGuest         = "Guest"
	syncUsersJobName             = "sync_users"
	metricsJobName               = "metrics"
	checkCredentialsJobName      = "check_credentials" //#nosec G101 -- This is a false positive

	updateMetricsTaskFrequency = 15 * time.Minute
	metricsActiveUsersRange    = 7 * 24 * time.Hour
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	msteamsAppClientMutex sync.RWMutex
	msteamsAppClient      msteams.Client

	stopSubscriptions func()
	stopContext       context.Context

	botUserID string
	remoteID  string
	apiClient *pluginapi.Client

	store                     store.Store
	subscriptionsClusterMutex *cluster.Mutex
	connectClusterMutex       *cluster.Mutex
	monitor                   *monitor.Monitor
	syncUserJob               *cluster.Job
	checkCredentialsJob       *cluster.Job
	apiHandler                *API

	activityHandler *handlers.ActivityHandler

	clientBuilderWithToken func(string, string, string, string, *oauth2.Token, *pluginapi.LogService) msteams.Client
	metricsService         metrics.Metrics
	metricsHandler         http.Handler
	metricsJob             *cluster.Job

	subCommands      []string
	subCommandsMutex sync.RWMutex
}

func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.apiHandler.ServeHTTP(w, r)
}

func (p *Plugin) ServeMetrics(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.metricsHandler.ServeHTTP(w, r)
}

func (p *Plugin) GetAPI() plugin.API {
	return p.API
}

func (p *Plugin) GetMetrics() metrics.Metrics {
	return p.metricsService
}

func (p *Plugin) GetStore() store.Store {
	return p.store
}

func (p *Plugin) GetSyncDirectMessages() bool {
	return p.getConfiguration().SyncDirectMessages
}

func (p *Plugin) GetSyncGroupMessages() bool {
	return p.getConfiguration().SyncGroupMessages
}

func (p *Plugin) GetSyncLinkedChannels() bool {
	return p.getConfiguration().SyncLinkedChannels
}

func (p *Plugin) GetSyncReactions() bool {
	return p.getConfiguration().SyncReactions
}

func (p *Plugin) GetSyncFileAttachments() bool {
	return p.getConfiguration().SyncFileAttachments
}

func (p *Plugin) GetSyncGuestUsers() bool {
	return p.getConfiguration().SyncGuestUsers
}

func (p *Plugin) GetMaxSizeForCompleteDownload() int {
	return p.getConfiguration().MaxSizeForCompleteDownload
}

func (p *Plugin) GetBufferSizeForStreaming() int {
	return p.getConfiguration().BufferSizeForFileStreaming
}

func (p *Plugin) GetBotUserID() string {
	return p.botUserID
}

func (p *Plugin) GetSelectiveSync() bool {
	return p.getConfiguration().SelectiveSync
}

func (p *Plugin) MessageFingerprint() string {
	return "<abbr title=\"generated-from-mattermost\"></abbr>"
}

func (p *Plugin) GetClientForApp() msteams.Client {
	p.msteamsAppClientMutex.RLock()
	defer p.msteamsAppClientMutex.RUnlock()

	return p.msteamsAppClient
}

func (p *Plugin) GetURL() string {
	config := p.API.GetConfig()
	siteURL := ""
	if config.ServiceSettings.SiteURL != nil {
		siteURL = *config.ServiceSettings.SiteURL
	}
	if strings.HasSuffix(siteURL, "/") {
		return siteURL + "plugins/" + pluginID
	}
	return siteURL + "/plugins/" + pluginID
}

func (p *Plugin) OnDisconnectedTokenHandler(userID string) {
	p.API.LogInfo("Token for user disconnected", "user_id", userID)
	p.metricsService.ObserveOAuthTokenInvalidated()

	teamsUserID, err := p.store.MattermostToTeamsUserID(userID)
	if err != nil {
		p.API.LogWarn("Unable to get teams user id from mattermost to user", "user_id", userID, "error", err.Error())
		return
	}
	if err2 := p.store.SetUserInfo(userID, teamsUserID, nil); err2 != nil {
		p.API.LogWarn("Unable clean invalid token for the user", "user_id", userID, "error", err2.Error())
		return
	}
	channel, appErr := p.API.GetDirectChannel(userID, p.GetBotUserID())
	if appErr != nil {
		p.API.LogWarn("Unable to get direct channel for send message to user", "user_id", userID, "error", appErr.Error())
		return
	}

	message := "Your connection to Microsoft Teams has been lost. "
	p.SendConnectMessage(channel.Id, userID, message)
}

func (p *Plugin) GetClientForUser(userID string) (msteams.Client, error) {
	token, _ := p.store.GetTokenForMattermostUser(userID)
	if token == nil {
		return nil, errors.New("not connected user")
	}

	client := p.clientBuilderWithToken(p.GetURL()+"/oauth-redirect", p.getConfiguration().TenantID, p.getConfiguration().ClientID, p.getConfiguration().ClientSecret, token, &p.apiClient.Log)
	client = client_timerlayer.New(client, p.GetMetrics())
	client = client_disconnectionlayer.New(client, userID, p.OnDisconnectedTokenHandler)

	if token.Expiry.Before(time.Now()) {
		newToken, err := client.RefreshToken(token)
		if err != nil {
			return nil, err
		}
		teamsUserID, err := p.store.MattermostToTeamsUserID(userID)
		if err != nil {
			return nil, err
		}
		if err := p.store.SetUserInfo(userID, teamsUserID, newToken); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func (p *Plugin) GetClientForTeamsUser(teamsUserID string) (msteams.Client, error) {
	userID, err := p.store.TeamsToMattermostUserID(teamsUserID)
	if err != nil {
		return nil, err
	}

	return p.GetClientForUser(userID)
}

func (p *Plugin) connectTeamsAppClient() error {
	p.msteamsAppClientMutex.Lock()
	defer p.msteamsAppClientMutex.Unlock()

	// We don't currently support reconnecting with a new configuration: a plugin restart is
	// required.
	if p.msteamsAppClient != nil {
		return nil
	}

	msteamsAppClient := msteams.NewApp(
		p.getConfiguration().TenantID,
		p.getConfiguration().ClientID,
		p.getConfiguration().ClientSecret,
		&p.apiClient.Log,
	)

	p.msteamsAppClient = client_timerlayer.New(msteamsAppClient, p.GetMetrics())
	err := p.msteamsAppClient.Connect()
	if err != nil {
		p.API.LogError("Unable to connect to the app client", "error", err)
		return err
	}
	return nil
}

func (p *Plugin) start(isRestart bool) {
	var err error

	if !isRestart {
		p.metricsJob, err = cluster.Schedule(
			p.API,
			metricsJobName,
			cluster.MakeWaitForRoundedInterval(updateMetricsTaskFrequency),
			p.updateMetrics,
		)
		if err != nil {
			p.API.LogError("failed to start metrics job", "error", err)
		}
	}

	p.metricsService.ObserveWhitelistLimit(p.configuration.ConnectedUsersAllowed)

	// We don't restart the activity handler since it's stateless.
	if !isRestart {
		p.activityHandler.Start()
	}

	err = p.connectTeamsAppClient()
	if err != nil {
		return
	}

	p.monitor = monitor.New(p.GetClientForApp(), p.store, p.API, p.GetMetrics(), p.GetURL()+"/", p.getConfiguration().WebhookSecret, p.getConfiguration().EvaluationAPI, p.getBase64Certificate(), p.GetSyncDirectMessages(), p.GetSyncGroupMessages())
	if err = p.monitor.Start(); err != nil {
		p.API.LogError("Unable to start the monitoring system", "error", err.Error())
	}

	ctx, stop := context.WithCancel(context.Background())
	p.stopSubscriptions = stop
	p.stopContext = ctx

	if p.getConfiguration().SyncUsers > 0 {
		p.API.LogInfo("Starting the sync users job")

		// Close the previous background job if exists.
		p.stopSyncUsersJob()

		// Start syncing the users on plugin start. The below job just schedules the job to run at a given interval of time but does not run it while scheduling. To avoid this, we call the function once separately to sync the users.
		p.syncUsers()

		job, jobErr := cluster.Schedule(
			p.API,
			syncUsersJobName,
			cluster.MakeWaitForRoundedInterval(time.Duration(p.getConfiguration().SyncUsers)*time.Minute),
			p.syncUsersPeriodically,
		)
		if jobErr != nil {
			p.API.LogError("error in scheduling the sync users job", "error", jobErr)
			return
		}

		p.syncUserJob = job
	}

	if !p.getConfiguration().DisableCheckCredentials {
		checkCredentialsJob, jobErr := cluster.Schedule(
			p.API,
			checkCredentialsJobName,
			cluster.MakeWaitForRoundedInterval(24*time.Hour),
			p.checkCredentials,
		)
		if jobErr != nil {
			p.API.LogError("error in scheduling the check credentials job", "error", jobErr)
			return
		}
		p.checkCredentialsJob = checkCredentialsJob

		// Run the job above right away so we immediately populate metrics.
		p.checkCredentials()
	}

	// Unregister and re-register slash command to reflect any configuration changes.
	if err = p.API.UnregisterCommand("", "msteams"); err != nil {
		p.API.LogWarn("Failed to unregister command", "error", err)
	}
	if err = p.API.RegisterCommand(p.createCommand(p.getConfiguration().SyncLinkedChannels)); err != nil {
		p.API.LogError("Failed to register command", "error", err)
	}
	p.API.LogDebug("plugin started")
}

func (p *Plugin) getBase64Certificate() string {
	certificate := p.getConfiguration().CertificatePublic
	if certificate == "" {
		return ""
	}
	block, _ := pem.Decode([]byte(certificate))
	if block == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(pem.EncodeToMemory(block))
}

func (p *Plugin) getPrivateKey() (*rsa.PrivateKey, error) {
	keyPemString := p.getConfiguration().CertificateKey
	if keyPemString == "" {
		return nil, errors.New("certificate private key not configured")
	}
	privPem, _ := pem.Decode([]byte(keyPemString))
	if privPem == nil {
		return nil, errors.New("invalid certificate key")
	}
	var err error
	var parsedKey any
	if parsedKey, err = x509.ParsePKCS8PrivateKey(privPem.Bytes); err != nil { // note this returns type `interface{}`
		return nil, err
	}

	var privateKey *rsa.PrivateKey
	var ok bool
	privateKey, ok = parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("Not valid key")
	}

	return privateKey, nil
}

func (p *Plugin) stop(isRestart bool) {
	if p.monitor != nil {
		p.monitor.Stop()
	}
	if p.stopSubscriptions != nil {
		p.stopSubscriptions()
		time.Sleep(1 * time.Second)
	}

	// We don't stop the activity handler on restart since it's stateless.
	if !isRestart && p.activityHandler != nil {
		p.activityHandler.Stop()
	}

	p.stopSyncUsersJob()

	if p.checkCredentialsJob != nil {
		if err := p.checkCredentialsJob.Close(); err != nil {
			p.API.LogError("Failed to close background check credentials job", "error", err)
		}
		p.checkCredentialsJob = nil
	}

	if !isRestart && p.metricsJob != nil {
		if err := p.metricsJob.Close(); err != nil {
			p.API.LogError("failed to close metrics job", "error", err)
		}
	}

	if !isRestart {
		if err := p.apiClient.Store.Close(); err != nil {
			p.API.LogError("failed to close db connection", "error", err)
		}
	}
}

func (p *Plugin) restart() {
	p.stop(true)
	p.start(true)
}

func (p *Plugin) generatePluginSecrets() error {
	needSaveConfig := false
	cfg := p.getConfiguration().Clone()
	if cfg.WebhookSecret == "" {
		secret, err := generateSecret()
		if err != nil {
			return err
		}

		cfg.WebhookSecret = secret
		needSaveConfig = true
	}
	if cfg.EncryptionKey == "" {
		secret, err := generateSecret()
		if err != nil {
			return err
		}

		cfg.EncryptionKey = secret
		needSaveConfig = true
	}
	if needSaveConfig {
		configMap, err := cfg.ToMap()
		if err != nil {
			return err
		}
		p.setConfiguration(cfg)
		if appErr := p.API.SavePluginConfig(configMap); appErr != nil {
			return appErr
		}
	}
	return nil
}

func (p *Plugin) onActivate() error {
	if p.clientBuilderWithToken == nil {
		p.clientBuilderWithToken = msteams.NewTokenClient
	}
	err := p.generatePluginSecrets()
	if err != nil {
		return err
	}

	p.metricsService = metrics.NewMetrics(metrics.InstanceInfo{
		InstallationID: os.Getenv("MM_CLOUD_INSTALLATION_ID"),
		PluginVersion:  manifest.Version,
	})
	p.metricsHandler = metrics.NewMetricsHandler(p.GetMetrics())

	p.apiClient = pluginapi.NewClient(p.API, p.Driver)

	config := p.apiClient.Configuration.GetConfig()
	license := p.apiClient.System.GetLicense()
	if !pluginapi.IsE20LicensedOrDevelopment(config, license) {
		return errors.New("this plugin requires an enterprise license")
	}

	p.activityHandler = handlers.New(p)

	p.subscriptionsClusterMutex, err = cluster.NewMutex(p.API, subscriptionsClusterMutexKey)
	if err != nil {
		return err
	}

	p.connectClusterMutex, err = cluster.NewMutex(p.API, connectClusterMutexKey)
	if err != nil {
		return err
	}

	p.botUserID, err = p.apiClient.Bot.EnsureBot(&model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: "Created by the MS Teams Sync plugin.",
	}, pluginapi.ProfileImageBytes(assets.Icon))
	if err != nil {
		return err
	}

	p.remoteID, err = p.API.RegisterPluginForSharedChannels(model.RegisterPluginOpts{
		Displayname:  pluginID,
		PluginID:     pluginID,
		CreatorID:    p.botUserID,
		AutoShareDMs: false,
		AutoInvited:  true,
	})
	if err != nil {
		return err
	}
	p.API.LogInfo("Registered plugin for shared channels", "remote_id", p.remoteID)

	if p.getConfiguration().DisableSyncMsg {
		p.API.LogInfo("Unregistering plugin for shared channels since sync msg disabled")
		if err = p.API.UnregisterPluginForSharedChannels(pluginID); err != nil {
			p.API.LogWarn("Unable to unregister plugin for shared channels", "error", err)
		}
	}

	if p.store == nil {
		if p.apiClient.Store.DriverName() != model.DatabaseDriverPostgres {
			return fmt.Errorf("unsupported database driver: %s", p.apiClient.Store.DriverName())
		}

		db, dbErr := p.apiClient.Store.GetMasterDB()
		if dbErr != nil {
			return dbErr
		}

		replica, repErr := p.apiClient.Store.GetReplicaDB()
		if repErr != nil {
			return repErr
		}

		store := sqlstore.New(
			db,
			replica,
			p.API,
			func() []string { return strings.Split(p.configuration.EnabledTeams, ",") },
			func() []byte { return []byte(p.configuration.EncryptionKey) },
		)
		p.store = timerlayer.New(store, p.GetMetrics())

		if err = p.store.Init(p.remoteID); err != nil {
			return err
		}
	}

	if !p.getConfiguration().DisableSyncMsg {
		linkedChannels, err := p.store.ListChannelLinks()
		if err != nil {
			p.API.LogError("Failed to list channel links for shared channels", "error", err.Error())
			return err
		}
		for _, linkedChannel := range linkedChannels {
			_, err = p.API.ShareChannel(&model.SharedChannel{
				ChannelId: linkedChannel.MattermostChannelID,
				TeamId:    linkedChannel.MattermostTeamID,
				Home:      true,
				ReadOnly:  false,
				CreatorId: p.botUserID,
				RemoteId:  p.remoteID,
				ShareName: linkedChannel.MattermostChannelID,
			})
			if err != nil {
				p.API.LogWarn("Unable to share channel", "error", err, "channelID", linkedChannel.MattermostChannelID, "teamID", linkedChannel.MattermostTeamID, "remoteID", p.remoteID)
			}

			p.API.LogInfo("Shared previously linked channel", "channel_id", linkedChannel.MattermostChannelID)
		}
	}

	p.apiHandler = NewAPI(p, p.store)

	if err := p.validateConfiguration(p.getConfiguration()); err != nil {
		return err
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				p.GetMetrics().ObserveGoroutineFailure()
				p.API.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
			}
		}()
	}()

	go p.start(false)
	return nil
}

func (p *Plugin) OnActivate() error {
	if err := p.onActivate(); err != nil {
		p.API.LogWarn("error activating the plugin", "error", err)
		if p.store != nil {
			if err = p.apiClient.Store.Close(); err != nil {
				p.API.LogWarn("failed to close db connection", "error", err)
			}
		}
		return err
	}
	return nil
}

func (p *Plugin) OnDeactivate() error {
	p.stop(false)
	return nil
}

func (p *Plugin) syncUsersPeriodically() {
	defer func() {
		if r := recover(); r != nil {
			p.GetMetrics().ObserveGoroutineFailure()
			p.API.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	p.API.LogInfo("Running the Sync Users Job")
	p.syncUsers()
}

func (p *Plugin) stopSyncUsersJob() {
	if p.syncUserJob != nil {
		if err := p.syncUserJob.Close(); err != nil {
			p.API.LogError("Failed to close background sync users job", "error", err)
		}
	}
}

func (p *Plugin) syncUsers() {
	done := p.GetMetrics().ObserveWorker(metrics.WorkerSyncUsers)
	defer done()

	// Get the users registered in MS Teams
	msUsers, err := p.GetClientForApp().ListUsers()
	if err != nil {
		p.API.LogWarn("Unable to list MS Teams users during sync user job", "error", err.Error())
		return
	}

	mmUsers, appErr := p.API.GetUsers(&model.UserGetOptions{Page: 0, PerPage: math.MaxInt32})
	if appErr != nil {
		p.API.LogWarn("Unable to get MM users during sync user job", "error", appErr.Error())
		return
	}

	// Map MM users with MS Teams users
	mmUsersMap := make(map[string]*model.User, len(mmUsers))
	for _, u := range mmUsers {
		mmUsersMap[u.Email] = u
	}

	configuration := p.getConfiguration()
	syncGuestUsers := configuration.SyncGuestUsers
	var activeMSTeamsUsersCount int64
	for _, msUser := range msUsers {
		if msUser.IsAccountEnabled {
			activeMSTeamsUsersCount++
		}

		userSuffixID := 1

		// The email field is mandatory in MM, if there is no email we skip the user
		if msUser.Mail == "" {
			continue
		}

		// Determine if the user is already present
		mmUser, isUserPresent := mmUsersMap[msUser.Mail]

		// Set the authData if promoting syntetic users
		authData := ""
		if configuration.AutomaticallyPromoteSyntheticUsers {
			switch configuration.SyntheticUserAuthData {
			case "ID":
				authData = msUser.ID
			case "Mail":
				authData = msUser.Mail
			case "UserPrincipalName":
				authData = msUser.UserPrincipalName
			}
		}

		username := "msteams_" + slug.Make(msUser.DisplayName)
		if isUserPresent {
			// Update the user if needed
			if p.IsRemoteUser(mmUser) {
				if !syncGuestUsers && msUser.Type == msteamsUserTypeGuest {
					if mmUser.DeleteAt == 0 {
						// if the user is a guest and should not sync, deactivate it
						p.API.LogInfo("Deactivating the guest user account", "user_id", mmUser.Id, "teams_user_id", msUser.ID)
						if err := p.API.UpdateUserActive(mmUser.Id, false); err != nil {
							p.API.LogWarn("Unable to deactivate the guest user account", "user_id", mmUser.Id, "teams_user_id", msUser.ID, "error", err.Error())
						}
					}
					continue
				}

				if teamsUserID, _ := p.store.MattermostToTeamsUserID(mmUser.Id); teamsUserID == "" {
					if err = p.store.SetUserInfo(mmUser.Id, msUser.ID, nil); err != nil {
						p.API.LogWarn("Unable to store Mattermost user ID vs Teams user ID in sync user job", "user_id", mmUser.Id, "teams_user_id", msUser.ID, "error", err.Error())
					}
				}

				if msUser.IsAccountEnabled {
					// Activate the deactivated Mattermost user corresponding to the MS Teams user.
					if mmUser.DeleteAt != 0 {
						p.API.LogInfo("Activating the inactive user", "user_id", mmUser.Id, "teams_user_id", msUser.ID, "type", msUser.Type)
						if err := p.API.UpdateUserActive(mmUser.Id, true); err != nil {
							p.API.LogWarn("Unable to activate the user", "user_id", mmUser.Id, "teams_user_id", msUser.ID, "error", err.Error())
						}
					}
				} else {
					// Deactivate the active Mattermost user corresponding to the MS Teams user.
					if mmUser.DeleteAt == 0 {
						p.API.LogInfo("Deactivating the Mattermost user account", "user_id", mmUser.Id, "teams_user_id", msUser.ID, "type", msUser.Type)
						if err := p.API.UpdateUserActive(mmUser.Id, false); err != nil {
							p.API.LogWarn("Unable to deactivate the Mattermost user account", "user_id", mmUser.Id, "teams_user_id", msUser.ID, "error", err.Error())
						}
					}

					continue
				}

				if configuration.AutomaticallyPromoteSyntheticUsers {
					// We need to retrieve the user individually because `GetUsers` does not return AuthData
					user, err := p.API.GetUser(mmUser.Id)
					if err != nil {
						p.API.LogWarn("Unable to fetch MM user", "user_id", mmUser.Id, "teams_user_id", msUser.ID, "error", err.Error())
						continue
					}

					// Update AuthService/AuthData if it changed
					if mmUser.AuthService != configuration.SyntheticUserAuthService || (user.AuthData != nil && authData != "" && *user.AuthData != authData) {
						p.API.LogInfo("Updating user auth service", "user_id", mmUser.Id, "teams_user_id", msUser.ID, "auth_service", configuration.SyntheticUserAuthService)
						if _, err := p.API.UpdateUserAuth(mmUser.Id, &model.UserAuth{
							AuthService: configuration.SyntheticUserAuthService,
							AuthData:    &authData,
						}); err != nil {
							p.API.LogWarn("Unable to update user auth service during sync user job", "user_id", mmUser.Id, "teams_user_id", msUser.ID, "error", err.Error())
						}
					}
				}

				// Update the user profile if needed
				shouldUpdate := false
				if !strings.HasPrefix(mmUser.Username, "msteams_") && username != mmUser.Username {
					mmUser.Username = username
					shouldUpdate = true
				}

				if mmUser.FirstName != msUser.DisplayName {
					mmUser.FirstName = msUser.DisplayName
					shouldUpdate = true
				}

				if shouldUpdate {
					for {
						p.API.LogInfo("Updating user profile", "user_id", mmUser.Id, "teams_user_id", msUser.ID)
						_, err := p.API.UpdateUser(mmUser)
						if err != nil {
							if err.Id == "app.user.save.username_exists.app_error" {
								// When there is already a user with the same username, start using the suffix
								mmUser.Username = username + "-" + fmt.Sprint(userSuffixID)
								userSuffixID++
								continue
							}

							p.API.LogWarn("Unable to update user during sync user job", "user_id", mmUser.Id, "teams_user_id", msUser.ID, "error", err.Error())
							break
						}

						break
					}
				}
			}
		} else if !msUser.IsAccountEnabled {
			continue
		} else {
			// If we are not sync'ing guests, but the user is a MS Team guest, deactivate it from the get go
			deleteAt := int64(0)
			if !syncGuestUsers && msUser.Type == msteamsUserTypeGuest {
				deleteAt = model.GetMillis()
			}

			newMMUser := &model.User{
				Email:         msUser.Mail,
				RemoteId:      &p.remoteID,
				FirstName:     msUser.DisplayName,
				Username:      username,
				EmailVerified: true,
				DeleteAt:      deleteAt,
			}

			if configuration.AutomaticallyPromoteSyntheticUsers && authData != "" {
				p.API.LogInfo("Creating new synthetic user", "teams_user_id", msUser.ID, "auth_service", configuration.SyntheticUserAuthService, "as_guest", msUser.Type == msteamsUserTypeGuest)
				newMMUser.AuthService = configuration.SyntheticUserAuthService
				newMMUser.AuthData = &authData
			} else {
				p.API.LogInfo("Creating new synthetic user", "teams_user_id", msUser.ID, "as_guest", msUser.Type == msteamsUserTypeGuest)
				newMMUser.Password = p.GenerateRandomPassword()
			}

			newMMUser.SetDefaultNotifications()
			newMMUser.NotifyProps[model.EmailNotifyProp] = "false"

			var newUser *model.User
			for {
				newUser, appErr = p.API.CreateUser(newMMUser)
				if appErr != nil {
					if appErr.Id == "app.user.save.username_exists.app_error" {
						newMMUser.Username = fmt.Sprintf("%s-%d", username, userSuffixID)
						userSuffixID++
						continue
					}

					p.API.LogWarn("Unable to create new MM user during sync job", "teams_user_id", msUser.ID, "error", appErr.Error())
					break
				}

				break
			}

			if newUser == nil || newUser.Id == "" {
				continue
			}

			p.API.LogInfo("Created new synthetic user", "user_id", newUser.Id, "teams_user_id", msUser.ID)

			preferences := model.Preferences{model.Preference{
				UserId:   newUser.Id,
				Category: model.PreferenceCategoryNotifications,
				Name:     model.PreferenceNameEmailInterval,
				Value:    "0",
			}}
			if prefErr := p.API.UpdatePreferencesForUser(newUser.Id, preferences); prefErr != nil {
				p.API.LogWarn("Unable to disable email notifications for new user", "user_id", newUser.Id, "teams_user_id", msUser.ID, "error", prefErr.Error())
			}

			if err = p.store.SetUserInfo(newUser.Id, msUser.ID, nil); err != nil {
				p.API.LogWarn("Unable to set user info during sync user job", "user_id", newUser.Id, "teams_user_id", msUser.ID, "error", err.Error())
			}
		}
	}
	p.GetMetrics().ObserveUpstreamUsers(activeMSTeamsUsersCount)
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
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(characterSet))))
		randomString.WriteString(string(characterSet[num.Int64()]))
	}

	return randomString.String()
}

// IsRemoteUser returns true if the given user is a remote user managed by this plugin.
func (p *Plugin) IsRemoteUser(user *model.User) bool {
	return user.RemoteId != nil && *user.RemoteId == p.remoteID
}

func (p *Plugin) GetRemoteID() string {
	return p.remoteID
}

func (p *Plugin) updateMetrics() {
	now := time.Now()
	p.API.LogInfo("Updating metrics")

	// it's a bit of a special case because it returns two values
	msTeamsPrimary, mmPrimary, primaryPlatformErr := p.store.GetUsersByPrimaryPlatformsCount(PreferenceCategoryPlugin)

	stats := []struct {
		name        string
		getData     func() (int64, error)
		observeData func(int64)
	}{
		{
			name:        "connecter users",
			getData:     p.store.GetConnectedUsersCount,
			observeData: p.GetMetrics().ObserveConnectedUsers,
		},
		{
			name:        "linked channels",
			getData:     p.store.GetLinkedChannelsCount,
			observeData: p.GetMetrics().ObserveLinkedChannels,
		},
		{
			name: "synthetic users",
			getData: func() (int64, error) {
				return p.store.GetSyntheticUsersCount(p.remoteID)
			},
			observeData: p.GetMetrics().ObserveSyntheticUsers,
		},
		{
			name:        "msteams primary users",
			getData:     func() (int64, error) { return msTeamsPrimary, primaryPlatformErr },
			observeData: p.GetMetrics().ObserveMSTeamsPrimary,
		},
		{
			name:        "mattermost primary users",
			getData:     func() (int64, error) { return mmPrimary, primaryPlatformErr },
			observeData: p.GetMetrics().ObserveMattermostPrimary,
		},
		{
			name:        "active users sending",
			getData:     func() (int64, error) { return p.store.GetActiveUsersSendingCount(metricsActiveUsersRange) },
			observeData: p.GetMetrics().ObserveActiveUsersSending,
		},
		{
			name:        "active users receiving",
			getData:     func() (int64, error) { return p.store.GetActiveUsersReceivingCount(metricsActiveUsersRange) },
			observeData: p.GetMetrics().ObserveActiveUsersReceiving,
		},
	}
	for _, stat := range stats {
		data, err := stat.getData()
		if err != nil {
			p.API.LogWarn("failed to get data for metric "+stat.name, "error", err)
			continue
		}

		stat.observeData(data)
	}

	p.API.LogInfo("Updating metrics done", "duration_ms", time.Since(now).Milliseconds())
}

func (p *Plugin) OnSharedChannelsPing(_ *model.RemoteCluster) bool {
	return true
}

func (p *Plugin) OnSharedChannelsAttachmentSyncMsg(fi *model.FileInfo, _ *model.Post, _ *model.RemoteCluster) error {
	now := model.GetMillis()

	isUpdate := fi.CreateAt != fi.UpdateAt
	isDelete := fi.DeleteAt != 0
	switch {
	case !isUpdate && !isDelete:
		p.GetMetrics().ObserveSyncMsgFileDelay(metrics.ActionCreated, now-fi.CreateAt)
	case isUpdate && !isDelete:
		p.GetMetrics().ObserveSyncMsgFileDelay(metrics.ActionUpdated, now-fi.UpdateAt)
	default:
		p.GetMetrics().ObserveSyncMsgFileDelay(metrics.ActionDeleted, now-fi.DeleteAt)
	}

	return nil
}

func (p *Plugin) OnSharedChannelsSyncMsg(msg *model.SyncMsg, _ *model.RemoteCluster) (model.SyncResponse, error) {
	now := model.GetMillis()

	var resp model.SyncResponse
	for _, post := range msg.Posts {
		isUpdate := post.CreateAt != post.UpdateAt
		isDelete := post.DeleteAt != 0

		switch {
		case !isUpdate && !isDelete:
			p.GetMetrics().ObserveSyncMsgPostDelay(metrics.ActionCreated, now-post.CreateAt)
		case isUpdate && !isDelete:
			p.GetMetrics().ObserveSyncMsgPostDelay(metrics.ActionUpdated, now-post.UpdateAt)
		default:
			p.GetMetrics().ObserveSyncMsgPostDelay(metrics.ActionDeleted, now-post.DeleteAt)
		}

		if resp.PostsLastUpdateAt < post.UpdateAt {
			resp.PostsLastUpdateAt = post.UpdateAt
		}
	}

	for _, reaction := range msg.Reactions {
		isUpdate := reaction.CreateAt != reaction.UpdateAt
		isDelete := reaction.DeleteAt != 0

		switch {
		case !isUpdate && !isDelete:
			p.GetMetrics().ObserveSyncMsgReactionDelay(metrics.ActionCreated, now-reaction.CreateAt)
		case isUpdate && !isDelete:
			p.GetMetrics().ObserveSyncMsgReactionDelay(metrics.ActionUpdated, now-reaction.UpdateAt)
		default:
			p.GetMetrics().ObserveSyncMsgReactionDelay(metrics.ActionDeleted, now-reaction.DeleteAt)
		}

		if resp.ReactionsLastUpdateAt < reaction.UpdateAt {
			resp.ReactionsLastUpdateAt = reaction.UpdateAt
		}
	}

	return resp, nil
}
