// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-msteams/assets"
	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
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
	"github.com/mattermost/mattermost/server/v8/channels/utils"
)

const (
	botUsername                  = "msteams"
	botDisplayName               = "MS Teams"
	pluginID                     = "com.mattermost.msteams-sync"
	subscriptionsClusterMutexKey = "subscriptions_cluster_mutex"
	connectClusterMutexKey       = "connect_cluster_mutex"
	msteamsUserTypeGuest         = "Guest"
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
	monitor                   *Monitor
	checkCredentialsJob       *cluster.Job
	apiHandler                *API

	activityHandler *ActivityHandler

	clientBuilderWithToken func(string, string, string, string, *oauth2.Token, *pluginapi.LogService) msteams.Client
	metricsService         metrics.Metrics
	metricsHandler         http.Handler
	metricsJob             *cluster.Job

	subCommands      []string
	subCommandsMutex sync.RWMutex

	cancelKeyFunc     context.CancelFunc
	cancelKeyFuncLock sync.Mutex
	tabAppJWTKeyFunc  keyfunc.Keyfunc
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

func (p *Plugin) GetTenantID() string {
	return p.getConfiguration().TenantID
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

func (p *Plugin) MessageFingerprint() string {
	return "<abbr title=\"generated-from-mattermost\"></abbr>"
}

func (p *Plugin) GetClientForApp() msteams.Client {
	p.msteamsAppClientMutex.RLock()
	defer p.msteamsAppClientMutex.RUnlock()

	return p.msteamsAppClient
}

func getURL(config *model.Config) string {
	siteURL := ""
	if config.ServiceSettings.SiteURL != nil {
		siteURL = *config.ServiceSettings.SiteURL
	}
	if !strings.HasSuffix(siteURL, "/") {
		siteURL += "/"
	}
	return siteURL + "plugins/" + pluginID
}

func (p *Plugin) GetURL() string {
	return getURL(p.API.GetConfig())
}

func getRelativeURL(config *model.Config) string {
	subpath, _ := utils.GetSubpathFromConfig(config)
	if !strings.HasSuffix(subpath, "/") {
		subpath += "/"
	}
	return subpath + "plugins/" + pluginID
}

func (p *Plugin) GetRelativeURL() string {
	return getRelativeURL(p.API.GetConfig())
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

		// Run the job above right away so we immediately populate metrics.
		go p.updateMetrics()
	}

	p.metricsService.ObserveConnectedUsersLimit(int64(p.configuration.ConnectedUsersAllowed))
	p.metricsService.ObservePendingInvitesLimit(int64(p.configuration.ConnectedUsersMaxPendingInvites))

	// We don't restart the activity handler since it's stateless.
	if !isRestart {
		p.activityHandler.Start()
	}

	err = p.connectTeamsAppClient()
	if err != nil {
		return
	}

	p.monitor = NewMonitor(p.GetClientForApp(), p.store, p.API, p.GetMetrics(), p.GetURL()+"/", p.getConfiguration().WebhookSecret, p.getConfiguration().EvaluationAPI)
	if err = p.monitor.Start(); err != nil {
		p.API.LogError("Unable to start the monitoring system", "error", err.Error())
	}

	ctx, stop := context.WithCancel(context.Background())
	p.stopSubscriptions = stop
	p.stopContext = ctx

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
		go p.checkCredentials()
	}

	// Unregister and re-register slash command to reflect any configuration changes.
	if err = p.API.UnregisterCommand("", "msteams"); err != nil {
		p.API.LogWarn("Failed to unregister command", "error", err)
	}
	if err = p.API.RegisterCommand(p.createCommand()); err != nil {
		p.API.LogError("Failed to register command", "error", err)
	}

	p.cancelKeyFuncLock.Lock()
	if !isRestart && p.cancelKeyFunc == nil {
		p.tabAppJWTKeyFunc, p.cancelKeyFunc = setupJWKSet()
	}
	p.cancelKeyFuncLock.Unlock()

	p.API.LogDebug("plugin started")
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

	if !isRestart {
		p.cancelKeyFuncLock.Lock()
		if p.cancelKeyFunc != nil {
			p.cancelKeyFunc()
			p.cancelKeyFunc = nil
		}
		p.cancelKeyFuncLock.Unlock()
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

	p.activityHandler = NewActivityHandler(p)

	p.subscriptionsClusterMutex, err = cluster.NewMutex(p.API, subscriptionsClusterMutexKey)
	if err != nil {
		return err
	}

	p.connectClusterMutex, err = cluster.NewMutex(p.API, connectClusterMutexKey)
	if err != nil {
		return err
	}

	logger := logrus.StandardLogger()
	pluginapi.ConfigureLogrus(logger, p.apiClient)

	p.botUserID, err = p.apiClient.Bot.EnsureBot(&model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: "Created by the MS Teams Sync plugin.",
	}, pluginapi.ProfileImageBytes(assets.Icon))
	if err != nil {
		return err
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
			func() []byte { return []byte(p.configuration.EncryptionKey) },
		)
		p.store = timerlayer.New(store, p.GetMetrics())

		if err = p.store.Init(p.remoteID); err != nil {
			return err
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

func (p *Plugin) IsUserConnected(userID string) (bool, error) {
	token, err := p.store.GetTokenForMattermostUser(userID)
	if err != nil && err != sql.ErrNoRows {
		return false, errors.Wrap(err, "Unable to determine if user is connected to MS Teams")
	}
	return token != nil, nil
}

func (p *Plugin) GetRemoteID() string {
	return p.remoteID
}

func (p *Plugin) updateMetrics() {
	defer func() {
		if r := recover(); r != nil {
			p.GetMetrics().ObserveGoroutineFailure()
			p.API.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	done := p.GetMetrics().ObserveWorker(metrics.WorkerMetricsUpdater)
	defer done()

	stats := []struct {
		name        string
		getData     func() (int64, error)
		observeData func(int64)
	}{
		{
			name:        "connected users",
			getData:     p.store.GetConnectedUsersCount,
			observeData: p.GetMetrics().ObserveConnectedUsers,
		},
		{
			name: "invited users",
			getData: func() (int64, error) {
				val, err := p.store.GetInvitedCount()
				return int64(val), err
			},
			observeData: p.GetMetrics().ObservePendingInvites,
		},
		{
			name: "whitelisted users",
			getData: func() (int64, error) {
				val, err := p.store.GetWhitelistCount()
				return int64(val), err
			},
			observeData: p.GetMetrics().ObserveWhitelistedUsers,
		},
		{
			name:        "linked channels",
			getData:     p.store.GetLinkedChannelsCount,
			observeData: p.GetMetrics().ObserveLinkedChannels,
		},
		{
			name:        "active users",
			getData:     func() (int64, error) { return p.store.GetActiveUsersCount(metricsActiveUsersRange) },
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
}
