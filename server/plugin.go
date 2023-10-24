package main

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gosimple/slug"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/cluster"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/handlers"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/monitor"
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
	msteamsUserTypeGuest  = "Guest"
	syncUsersJobName      = "sync_users"

	metricsExposePort          = ":9094"
	updateMetricsTaskFrequency = 15 * time.Minute

	discardedReasonUnableToGetMMData         = "unable_to_get_mm_data"
	discardedReasonUnableToUploadFileOnTeams = "unable_to_upload_file_on_teams"

	actionSourceMattermost = "mattermost"
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

	userID    string
	apiClient *pluginapi.Client

	store        store.Store
	clusterMutex *cluster.Mutex
	monitor      *monitor.Monitor
	syncUserJob  *cluster.Job

	activityHandler *handlers.ActivityHandler

	clientBuilderWithToken func(string, string, string, string, *oauth2.Token, *pluginapi.LogService) msteams.Client
	metricsService         metrics.Metrics
	metricsServer          *metrics.Server
}

func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	api := NewAPI(p, p.store)
	api.ServeHTTP(w, r)
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
	return p.clientBuilderWithToken(p.GetURL()+"/oauth-redirect", p.getConfiguration().TenantID, p.getConfiguration().ClientID, p.getConfiguration().ClientSecret, token, &p.apiClient.Log), nil
}

func (p *Plugin) GetClientForTeamsUser(teamsUserID string) (msteams.Client, error) {
	token, _ := p.store.GetTokenForMSTeamsUser(teamsUserID)
	if token == nil {
		return nil, errors.New("not connected user")
	}

	return p.clientBuilderWithToken(p.GetURL()+"/oauth-redirect", p.getConfiguration().TenantID, p.getConfiguration().ClientID, p.getConfiguration().ClientSecret, token, &p.apiClient.Log), nil
}

func (p *Plugin) connectTeamsAppClient() error {
	p.msteamsAppClientMutex.Lock()
	defer p.msteamsAppClientMutex.Unlock()

	if p.msteamsAppClient == nil {
		p.msteamsAppClient = msteams.NewApp(
			p.getConfiguration().TenantID,
			p.getConfiguration().ClientID,
			p.getConfiguration().ClientSecret,
			&p.apiClient.Log,
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
	enableMetrics := p.API.GetConfig().MetricsSettings.Enable

	if enableMetrics != nil && *enableMetrics {
		p.metricsService = metrics.NewMetrics(metrics.InstanceInfo{
			InstallationID: os.Getenv("MM_CLOUD_INSTALLATION_ID"),
		})

		// run metrics server to expose data
		p.runMetricsServer()
		// run metrics updater recurring task
		p.runMetricsUpdaterTask(p.store, updateMetricsTaskFrequency)
	}

	p.activityHandler.Start()

	err := p.connectTeamsAppClient()
	if err != nil {
		return
	}

	p.monitor = monitor.New(p.msteamsAppClient, p.store, p.API, p.GetURL()+"/", p.getConfiguration().WebhookSecret, p.getConfiguration().EvaluationAPI)
	if err = p.monitor.Start(); err != nil {
		p.API.LogError("Unable to start the monitoring system", "error", err.Error())
	}

	ctx, stop := context.WithCancel(context.Background())
	p.stopSubscriptions = stop
	p.stopContext = ctx
	if syncSince != nil {
		go p.syncSince(*syncSince)
	}

	if p.getConfiguration().SyncUsers > 0 {
		p.API.LogDebug("Starting the sync users job")

		// Close the previous background job if exists.
		p.stopSyncUsersJob()

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
		if sErr := p.store.SetJobStatus(syncUsersJobName, false); sErr != nil {
			p.API.LogError("error in setting the sync users job status", "error", sErr.Error())
		}
	}
}

func (p *Plugin) syncSince(syncSince time.Time) {
	// TODO: Implement the sync mechanism
	p.API.LogDebug("Syncing since", "date", syncSince)
}

func (p *Plugin) stop() {
	if p.monitor != nil {
		p.monitor.Stop()
	}
	if p.metricsServer != nil {
		if err := p.metricsServer.Shutdown(); err != nil {
			p.API.LogWarn("Error shutting down metrics server", "error", err)
		}
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

	p.apiClient = pluginapi.NewClient(p.API, p.Driver)

	p.activityHandler = handlers.New(p)

	clusterMutex, err := cluster.NewMutex(p.API, clusterMutexKey)
	if err != nil {
		return err
	}
	botID, err := p.apiClient.Bot.EnsureBot(&model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: "Created by the MS Teams Sync plugin.",
	}, pluginapi.ProfileImagePath("assets/msteams-sync-icon.png"))
	if err != nil {
		return err
	}
	p.userID = botID
	p.clusterMutex = clusterMutex

	if err = p.API.RegisterCommand(p.createMsteamsSyncCommand()); err != nil {
		return err
	}

	if p.store == nil {
		db, dbErr := p.apiClient.Store.GetMasterDB()
		if dbErr != nil {
			return dbErr
		}

		p.store = store.New(
			db,
			p.apiClient.Store.DriverName(),
			p.API,
			func() []string { return strings.Split(p.configuration.EnabledTeams, ",") },
			func() []byte { return []byte(p.configuration.EncryptionKey) },
		)
		if err = p.store.Init(); err != nil {
			return err
		}
	}

	whitelistSize, err := p.store.GetSizeOfWhitelist()
	if err != nil {
		return errors.New("failed to get the size of whitelist from the DB")
	}

	if p.getConfiguration().ConnectedUsersAllowed < whitelistSize {
		return errors.New("failed to save configuration, no. of connected users allowed should be greater than the current size of the whitelist")
	}

	go func() {
		if err := p.store.PrefillWhitelist(); err != nil {
			p.API.LogDebug("Error in populating the whitelist with already connected users", "Error", err.Error())
		}
	}()

	go p.start(lastRecivedChange)
	return nil
}

func (p *Plugin) OnDeactivate() error {
	p.stop()
	return nil
}

func (p *Plugin) syncUsersPeriodically() {
	defer func() {
		if sErr := p.store.SetJobStatus(syncUsersJobName, false); sErr != nil {
			p.API.LogDebug("Failed to set sync users job running status to false.")
		}
	}()

	isStatusUpdated, sErr := p.store.CompareAndSetJobStatus(syncUsersJobName, false, true)
	if sErr != nil {
		p.API.LogError("Something went wrong while fetching sync users job status", "Error", sErr.Error())
		return
	}

	if !isStatusUpdated {
		p.API.LogDebug("Sync users job already running")
		return
	}

	p.API.LogDebug("Running the Sync Users Job")
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
	msUsers, err := p.msteamsAppClient.ListUsers()
	if err != nil {
		p.API.LogError("Unable to list MS Teams users during sync user job", "error", err.Error())
		return
	}

	p.API.LogDebug("Count of MS Teams users", "count", len(msUsers))
	mmUsers, appErr := p.API.GetUsers(&model.UserGetOptions{Page: 0, PerPage: math.MaxInt32})
	if appErr != nil {
		p.API.LogError("Unable to get MM users during sync user job", "error", appErr.Error())
		return
	}

	p.API.LogDebug("Count of MM users", "count", len(mmUsers))
	mmUsersMap := make(map[string]*model.User, len(mmUsers))
	for _, u := range mmUsers {
		mmUsersMap[u.Email] = u
	}

	syncGuestUsers := p.getConfiguration().SyncGuestUsers
	for _, msUser := range msUsers {
		userSuffixID := 1
		if msUser.Mail == "" {
			continue
		}

		p.API.LogDebug("Running sync user job for user", "TeamsUserID", msUser.ID)

		mmUser, isUserPresent := mmUsersMap[msUser.Mail]

		if isUserPresent {
			if teamsUserID, _ := p.store.MattermostToTeamsUserID(mmUser.Id); teamsUserID == "" {
				if err = p.store.SetUserInfo(mmUser.Id, msUser.ID, nil); err != nil {
					p.API.LogDebug("Unable to store Mattermost user ID vs Teams user ID in sync user job", "MMUserID", mmUser.Id, "TeamsUserID", msUser.ID, "Error", err.Error())
				}
			}

			if isRemoteUser(mmUser) {
				if msUser.IsAccountEnabled {
					// Activate the deactivated Mattermost user corresponding to the MS Teams user.
					if mmUser.DeleteAt != 0 {
						p.API.LogDebug("Activating the inactive user", "TeamsUserID", msUser.ID)
						if err := p.API.UpdateUserActive(mmUser.Id, true); err != nil {
							p.API.LogError("Unable to activate the user", "MMUserID", mmUser.Id, "TeamsUserID", msUser.ID, "Error", err.Error())
						}
					}
				} else {
					// Deactivate the active Mattermost user corresponding to the MS Teams user.
					if mmUser.DeleteAt == 0 {
						p.API.LogDebug("Deactivating the Mattermost user account", "TeamsUserID", msUser.ID)
						if err := p.API.UpdateUserActive(mmUser.Id, false); err != nil {
							p.API.LogError("Unable to deactivate the Mattermost user account", "TeamsUserID", msUser.ID, "Error", err.Error())
						}
					}

					continue
				}
			}
		}

		if msUser.Type == msteamsUserTypeGuest {
			// Check if syncing of MS Teams guest users is disabled.
			if !syncGuestUsers {
				if isUserPresent && isRemoteUser(mmUser) {
					// Deactivate the Mattermost user corresponding to the MS Teams guest user.
					p.API.LogDebug("Deactivating the guest user account", "MMUserID", mmUser.Id, "TeamsUserID", msUser.ID)
					if err := p.API.UpdateUserActive(mmUser.Id, false); err != nil {
						p.API.LogError("Unable to deactivate the guest user account", "MMUserID", mmUser.Id, "TeamsUserID", msUser.ID, "Error", err.Error())
					}
				} else {
					// Skip syncing of MS Teams guest user.
					p.API.LogDebug("Skipping syncing of the guest user", "MMUserID", mmUser.Id, "TeamsUserID", msUser.ID)
				}

				continue
			}
		}

		username := "msteams_" + slug.Make(msUser.DisplayName)
		if !isUserPresent {
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
			newMMUser.SetDefaultNotifications()
			newMMUser.NotifyProps[model.EmailNotifyProp] = "false"

			var newUser *model.User
			for {
				newUser, appErr = p.API.CreateUser(newMMUser)
				if appErr != nil {
					if appErr.Id == "app.user.save.username_exists.app_error" {
						newMMUser.Username += "-" + fmt.Sprint(userSuffixID)
						userSuffixID++
						continue
					}

					p.API.LogError("Unable to create new MM user during sync job", "TeamsUserID", msUser.ID, "error", appErr.Error())
					break
				}

				break
			}

			if newUser == nil || newUser.Id == "" {
				continue
			}

			preferences := model.Preferences{model.Preference{
				UserId:   newUser.Id,
				Category: model.PreferenceCategoryNotifications,
				Name:     model.PreferenceNameEmailInterval,
				Value:    "0",
			}}
			if prefErr := p.API.UpdatePreferencesForUser(newUser.Id, preferences); prefErr != nil {
				p.API.LogError("Unable to disable email notifications for new user", "MMUserID", newUser.Id, "TeamsUserID", msUser.ID, "error", prefErr.Error())
			}

			if err = p.store.SetUserInfo(newUser.Id, msUser.ID, nil); err != nil {
				p.API.LogError("Unable to set user info during sync user job", "MMUserID", newUser.Id, "TeamsUserID", msUser.ID, "error", err.Error())
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

					p.API.LogError("Unable to update user during sync user job", "MMUserID", mmUser.Id, "TeamsUserID", msUser.ID, "error", err.Error())
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
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(characterSet))))
		randomString.WriteString(string(characterSet[num.Int64()]))
	}

	return randomString.String()
}

func isRemoteUser(user *model.User) bool {
	return user.RemoteId != nil && *user.RemoteId != "" && strings.HasPrefix(user.Username, "msteams_")
}

func (p *Plugin) runMetricsServer() {
	p.API.LogInfo("Starting metrics server", "port", metricsExposePort)

	p.metricsServer = metrics.NewMetricsServer(metricsExposePort, p.metricsService)

	// Run server to expose metrics
	go func() {
		err := p.metricsServer.Run()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.API.LogError("Metrics server could not be started", "error", err)
		}
	}()
}

func (p *Plugin) runMetricsUpdaterTask(store store.Store, updateMetricsTaskFrequency time.Duration) {
	metricsUpdater := func() {
		stats, err := store.GetStats()
		if err != nil {
			p.API.LogError("failed to update computed metrics", "error", err)
		}
		p.metricsService.ObserveConnectedUsersTotal(stats.ConnectedUsers)
		p.metricsService.ObserveSyntheticUsersTotal(stats.SyntheticUsers)
		p.metricsService.ObserveLinkedChannelsTotal(stats.LinkedChannels)
	}

	metricsUpdater()
	model.CreateRecurringTask("metricsUpdater", metricsUpdater, updateMetricsTaskFrequency)
}
