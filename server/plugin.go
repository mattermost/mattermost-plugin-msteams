package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
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
	msteamsUserTypeGuest  = "Guest"
	syncUsersJobName      = "sync_users"
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
	syncUserJob  *cluster.Job

	activityHandler *handlers.ActivityHandler

	clientBuilderWithToken func(string, string, string, string, *oauth2.Token, func(string, ...any)) msteams.Client
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

func (p *Plugin) GetSyncGuestUsers() bool {
	return p.getConfiguration().SyncGuestUsers
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
		return
	}

	p.monitor = monitor.New(p.msteamsAppClient, p.store, p.API, p.GetURL()+"/", p.getConfiguration().WebhookSecret, p.getConfiguration().EvaluationAPI)
	if err = p.monitor.Start(); err != nil {
		p.API.LogError("Unable to start the monitoring system", "error", err.Error())
	}

	ctx, stop := context.WithCancel(context.Background())
	p.stopSubscriptions = stop
	p.stopContext = ctx
	go p.startSubscriptions()
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

func (p *Plugin) startSubscriptions() {
	p.clusterMutex.Lock()
	defer p.clusterMutex.Unlock()

	if err := p.store.DeleteFakeSubscriptions(); err != nil {
		p.API.LogError("Unable to delete fake subscriptions", "error", err)
		return
	}

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

	mmChannelSubscriptions, err := p.store.ListChannelSubscriptions()
	if err != nil {
		p.API.LogError("Unable to list Mattermost channel subscriptions", "error", err)
		return
	}

	mmGlobalSubscriptions, err := p.store.ListGlobalSubscriptions()
	if err != nil {
		p.API.LogError("Unable to list Mattermost global subscriptions", "error", err)
		return
	}

	msteamsSubscriptionsMap, allChatsSubscription, err := p.monitor.GetMSTeamsSubscriptionsMap()
	if err != nil {
		p.API.LogError("Unable to list MS Teams subscriptions", "error", err)
		return
	}

	mmChannelSubscriptionsMap := make(map[string]storemodels.ChannelSubscription)
	for _, mmChannelSubscription := range mmChannelSubscriptions {
		mmChannelSubscriptionsMap[mmChannelSubscription.SubscriptionID] = mmChannelSubscription
	}

	wg := sync.WaitGroup{}
	ws := make(chan struct{}, 20)

	wg.Add(1)
	ws <- struct{}{}
	go func() {
		defer wg.Done()
		// Check if all chats subscription is present on Mattermost
		if len(mmGlobalSubscriptions) > 0 {
			mmGlobalSubscription := mmGlobalSubscriptions[0]
			// Check if all chats subscription is present on MS Teams
			if allChatsSubscription == nil {
				// Create all chats subscription on MS Teams
				if err = p.monitor.CreateAndSaveChatSubscription(&mmGlobalSubscription); err != nil {
					<-ws
					return
				}
			}
		} else {
			// Check if all chats subscription is present on MS Teams but not on Mattermost
			if allChatsSubscription != nil {
				if err = p.store.SaveGlobalSubscription(storemodels.GlobalSubscription{SubscriptionID: allChatsSubscription.ID, Type: "allChats", Secret: p.getConfiguration().WebhookSecret, ExpiresOn: allChatsSubscription.ExpiresOn}); err != nil {
					p.API.LogError("Unable to store all chats subscription", "subscriptionID", allChatsSubscription.ID, "error", err)
				}
				<-ws
				return
			}

			// Create all chats subscription
			if err = p.monitor.CreateAndSaveChatSubscription(nil); err != nil {
				<-ws
				return
			}
		}
		<-ws
	}()

	fakeSubscriptionCount := 1
	for _, link := range links {
		ws <- struct{}{}
		wg.Add(1)
		go func(link storemodels.ChannelLink) {
			defer wg.Done()
			mmSubscription, mmSubscriptionFound := mmChannelSubscriptionsMap[link.MattermostTeamID+link.MattermostChannelID]
			// Check if channel subscription is present for a link on Mattermost
			if mmSubscriptionFound {
				// Check if channel subscription is not present on MS Teams
				if _, msteamsSubscriptionFound := msteamsSubscriptionsMap[mmSubscription.SubscriptionID]; !msteamsSubscriptionFound {
					// Create channel subscription for the linked channel
					fakeSubscriptionCount, err = p.CreateChannelSubscriptions(link, &mmSubscription, fakeSubscriptionCount)
					if err != nil {
						<-ws
						return
					}
				} else if time.Until(mmSubscription.ExpiresOn) < (1 * time.Minute) {
					// Refresh the channel subscription
					expiresOn, rErr := p.msteamsAppClient.RefreshSubscription(mmSubscription.SubscriptionID)
					if rErr != nil {
						p.API.LogDebug("Unable to refresh the channel subscription", "subscriptionID", mmSubscription.SubscriptionID, "error", rErr)
						<-ws
						return
					}

					if err = p.store.UpdateSubscriptionExpiresOn(mmSubscription.SubscriptionID, *expiresOn); err != nil {
						p.API.LogDebug("Unable to store the refreshed channel subscription", "subscriptionID", mmSubscription.SubscriptionID, "error", err)
						<-ws
						return
					}
				}
			} else {
				// Create channel subscription for the linked channel
				fakeSubscriptionCount, err = p.CreateChannelSubscriptions(link, nil, fakeSubscriptionCount)
				if err != nil {
					<-ws
					return
				}
			}
			<-ws
		}(link)
	}
	wg.Wait()
	p.API.LogDebug("Starting subscriptions finished")
}

func (p *Plugin) CreateChannelSubscriptions(link storemodels.ChannelLink, mmSubscription *storemodels.ChannelSubscription, fakeSubscriptionCount int) (int, error) {
	// Create channel subscription for linked channel
	channelSubscription, err := p.msteamsAppClient.SubscribeToChannel(link.MSTeamsTeam, link.MSTeamsChannel, p.GetURL()+"/", p.getConfiguration().WebhookSecret)
	if err != nil {
		p.API.LogError("Unable to subscribe to channels", "channelID", link.MattermostChannelID, "error", err)
		if mmSubscription != nil {
			return fakeSubscriptionCount, err
		}

		// Mark this subscription to be created and retried by the monitor system
		_ = p.store.SaveChannelSubscription(storemodels.ChannelSubscription{
			SubscriptionID: "fake-subscription-id-" + fmt.Sprint(fakeSubscriptionCount),
			TeamID:         link.MSTeamsTeam,
			ChannelID:      link.MSTeamsChannel,
			ExpiresOn:      time.Now(),
			Secret:         p.getConfiguration().WebhookSecret,
		})
		fakeSubscriptionCount++
		return fakeSubscriptionCount, err
	}

	if mmSubscription != nil {
		if err = p.store.DeleteSubscription(mmSubscription.SubscriptionID); err != nil {
			p.API.LogError("Unable to delete the old channel subscription", "subscriptionID", mmSubscription.SubscriptionID, "error", err)
		}
	}

	if err = p.store.SaveChannelSubscription(storemodels.ChannelSubscription{
		SubscriptionID: channelSubscription.ID,
		TeamID:         link.MSTeamsTeam,
		ChannelID:      link.MSTeamsChannel,
		ExpiresOn:      channelSubscription.ExpiresOn,
		Secret:         p.getConfiguration().WebhookSecret,
	}); err != nil {
		p.API.LogError("Unable to save the channel subscription", "subscriptionID", channelSubscription.ID, "error", err)
		return fakeSubscriptionCount, err
	}

	p.API.LogDebug("Subscription to channel created", "subscriptionID", channelSubscription.ID, "teamID", link.MSTeamsTeam, "channelID", link.MSTeamsChannel)
	return fakeSubscriptionCount, nil
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

		p.API.LogDebug("Running sync user job for user with email", "email", msUser.Mail)

		mmUser, isUserPresent := mmUsersMap[msUser.Mail]

		if isUserPresent && isRemoteUser(mmUser) {
			if msUser.IsAccountEnabled {
				// Activate the deactived Mattermost user corresponding to MS Teams user.
				if mmUser.DeleteAt != 0 {
					p.API.LogDebug("Activating the inactive user", "Email", msUser.Mail)
					if err := p.API.UpdateUserActive(mmUser.Id, true); err != nil {
						p.API.LogError("Unable to activate the user", "Email", msUser.Mail, "Error", err.Error())
					}
				}
			} else {
				// Deactivate the active Mattermost user corresponding to MS Teams user.
				if mmUser.DeleteAt == 0 {
					p.API.LogDebug("Deactivating the Mattermost user account", "Email", msUser.Mail)
					if err := p.API.UpdateUserActive(mmUser.Id, false); err != nil {
						p.API.LogError("Unable to deactivate the Mattermost user account", "Email", mmUser.Email, "Error", err.Error())
					}
				}

				continue
			}
		}

		if msUser.Type == msteamsUserTypeGuest {
			// Check if syncing of MS Teams guest users is disabled.
			if !syncGuestUsers {
				if isUserPresent && isRemoteUser(mmUser) {
					// Deactivate the Mattermost user corresponding to the MS Teams guest user.
					p.API.LogDebug("Deactivating the guest user account", "Email", msUser.Mail)
					if err := p.API.UpdateUserActive(mmUser.Id, false); err != nil {
						p.API.LogError("Unable to deactivate the guest user account", "Email", mmUser.Email, "Error", err.Error())
					}
				} else {
					// Skip syncing of MS Teams guest user.
					p.API.LogDebug("Skipping syncing of the guest user", "Email", msUser.Mail)
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

					p.API.LogError("Unable to create new MM user during sync job", "email", msUser.Mail, "error", appErr.Error())
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
				p.API.LogError("Unable to disable email notifications for new user", "MMUserID", newUser.Id, "error", prefErr.Error())
			}

			if err = p.store.SetUserInfo(newUser.Id, msUser.ID, nil); err != nil {
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
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(characterSet))))
		randomString.WriteString(string(characterSet[num.Int64()]))
	}

	return randomString.String()
}

func isRemoteUser(user *model.User) bool {
	return user.RemoteId != nil && *user.RemoteId != "" && strings.HasPrefix(user.Username, "msteams_")
}
