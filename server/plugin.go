package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	msgraph "github.com/yaegashi/msgraph.go/beta"
	"github.com/yaegashi/msgraph.go/msauth"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

const (
	botUsername    = "msteams"
	botDisplayName = "MS Teams"
)

type ChannelLink struct {
	MattermostTeam    string
	MattermostChannel string
	MSTeamsTeam       string
	MSTeamsChannel    string
}

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	msteamsAppClientMutex sync.Mutex
	msteamsAppClient      *msgraph.GraphServiceRequestBuilder
	msteamsAppClientCtx   context.Context
	msteamsBotClientMutex sync.Mutex
	msteamsBotClient      *msgraph.GraphServiceRequestBuilder
	msteamsBotClientCtx   context.Context

	botID  string
	userID string

	subscriptionsToLinksMutex sync.Mutex
	subscriptionsToLinks      map[string]ChannelLink
	channelsLinked            map[string]ChannelLink

	stopSubscriptions func()
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	router := mux.NewRouter()
	router.HandleFunc("/avatar/{userId:.*}", p.getAvatar).Methods("GET")
	router.HandleFunc("/", p.processMessage).Methods("GET", "POST")
	router.ServeHTTP(w, r)
}

func (p *Plugin) connectTeamsAppClient() error {
	scopes := []string{"https://graph.microsoft.com/.default"}
	p.msteamsAppClientMutex.Lock()
	defer p.msteamsAppClientMutex.Unlock()
	if p.msteamsAppClient != nil {
		return nil
	}

	p.API.LogInfo("Connecting")
	ctx := context.Background()
	m := msauth.NewManager()
	sessionInfo, _ := p.API.KVGet("appSessionCache")
	if len(sessionInfo) > 0 {
		m.LoadBytes(sessionInfo)
	}
	ts, err := m.ClientCredentialsGrant(
		ctx,
		p.configuration.TenantId,
		p.configuration.ClientId,
		p.configuration.ClientSecret,
		scopes,
	)
	if err != nil {
		p.API.LogError("Couldn't start the session", "error", err)
		return err
	}
	sessionInfo, err = m.SaveBytes()
	if err != nil {
		p.API.LogError("Couldn't save the session", "error", err)
	}
	err = p.API.KVSet("appSessionCache", sessionInfo)
	if err != nil {
		p.API.LogError("Couldn't save the session", "error", err)
	}

	httpClient := oauth2.NewClient(ctx, ts)
	graphClient := msgraph.NewClient(httpClient)
	p.msteamsAppClient = graphClient
	p.msteamsAppClientCtx = ctx

	p.API.LogInfo("Connection succeeded")
	return nil
}

func (p *Plugin) connectTeamsBotClient() error {
	scopes := []string{"https://graph.microsoft.com/.default"}
	p.msteamsBotClientMutex.Lock()
	defer p.msteamsBotClientMutex.Unlock()
	if p.msteamsBotClient != nil {
		return nil
	}

	ctx := context.Background()
	m := msauth.NewManager()
	sessionInfo, _ := p.API.KVGet("botSessionCache")
	if len(sessionInfo) > 0 {
		m.LoadBytes(sessionInfo)
	}
	ts, err := m.ResourceOwnerPasswordGrant(
		ctx,
		p.configuration.TenantId,
		p.configuration.ClientId,
		p.configuration.ClientSecret,
		p.configuration.BotUsername,
		p.configuration.BotPassword,
		scopes,
	)
	if err != nil {
		return err
	}

	sessionInfo, err = m.SaveBytes()
	if err != nil {
		p.API.LogError("Couldn't save the session", "error", err)
	}
	err = p.API.KVSet("botSessionCache", sessionInfo)
	if err != nil {
		p.API.LogError("Couldn't save the session", "error", err)
	}

	httpClient := oauth2.NewClient(ctx, ts)
	graphClient := msgraph.NewClient(httpClient)
	p.msteamsBotClient = graphClient
	p.msteamsBotClientCtx = ctx

	req := graphClient.Me().Request()
	r, err := req.Get(ctx)
	if err != nil {
		return err
	}
	p.botID = *r.ID

	p.API.LogInfo("Connection succeeded")
	return nil
}

func (p *Plugin) start() {
	p.connectTeamsAppClient()
	p.connectTeamsBotClient()

	channelsLinkedData, appErr := p.API.KVGet("channelsLinked")
	if appErr != nil {
		p.API.LogError("Error getting the channels linked", "error", appErr)
		return
	}
	channelsLinked := map[string]ChannelLink{}
	err := json.Unmarshal(channelsLinkedData, &channelsLinked)
	if err != nil {
		p.API.LogError("Error getting the channels linked", "error", err)
		return
	}

	p.channelsLinked = channelsLinked
	p.subscriptionsToLinks = map[string]ChannelLink{}
	ctx, stop := context.WithCancel(context.Background())
	p.stopSubscriptions = stop
	err = p.clearSubscriptions()
	if err != nil {
		p.API.LogError("Unable to clear all subscriptions", "error", err)
	}
	for _, link := range channelsLinked {
		go p.subscribeToChannel(ctx, link)
	}
}

func (p *Plugin) stop() {
	if p.stopSubscriptions != nil {
		p.stopSubscriptions()
	}
}

func (p *Plugin) restart() {
	p.stop()
	p.start()
}

func (p *Plugin) OnActivate() error {
	p.stopSubscriptions = func() {}

	bot, appErr := p.API.CreateBot(&model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: "Created by the MS Teams Sync plugin.",
	})
	if appErr != nil {
		bot, err := p.API.GetUserByUsername(botUsername)
		if err != nil {
			return err
		}
		p.userID = bot.Id
	} else {
		p.userID = bot.UserId
	}

	if p.configuration.Config == "" {
		return nil
	}

	p.API.RegisterCommand(createMsteamsSyncCommand())

	p.start()

	return nil
}

func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	if post.Props != nil {
		if _, ok := post.Props["matterbridge_"+p.userID].(bool); ok {
			return
		}
	}

	channel, _ := p.API.GetChannel(post.ChannelId)

	link, ok := p.channelsLinked[channel.TeamId+":"+post.ChannelId]
	if !ok {
		return
	}

	user, _ := p.API.GetUser(post.UserId)

	go p.Send(link, user, post)
}

func (p *Plugin) OnDeactivate() error {
	p.stop()
	return nil
}

func (p *Plugin) Send(link ChannelLink, user *model.User, post *model.Post) (string, error) {
	p.API.LogDebug("\n\n\n=> Receiving message", "post", post)

	// TODO: Replace this with a template
	text := user.Username + "@mattermost: " + post.Message
	content := &msgraph.ItemBody{Content: &text}
	rmsg := &msgraph.ChatMessage{Body: content}

	parentID := []byte{}
	if post.RootId != "" {
		parentID, _ = p.API.KVGet("mattermost_teams_" + post.RootId)
	}

	var res *msgraph.ChatMessage
	if len(parentID) > 0 {
		var err error
		// TODO: Change the TEAMID and the CHANNELID for something that comes from the config somehow
		ct := p.msteamsBotClient.Teams().ID(link.MSTeamsTeam).Channels().ID(link.MSTeamsChannel).Messages().ID(string(parentID)).Replies().Request()
		res, err = ct.Add(p.msteamsBotClientCtx, rmsg)
		if err != nil {
			p.API.LogError("Error creating reply", "error", err)
			return "", err
		}
	} else {
		var err error
		// TODO: Change the TEAMID and the CHANNELID for something that comes from the config somehow
		ct := p.msteamsBotClient.Teams().ID(link.MSTeamsTeam).Channels().ID(link.MSTeamsChannel).Messages().Request()
		res, err = ct.Add(p.msteamsBotClientCtx, rmsg)
		if err != nil {
			p.API.LogError("Error creating message", "error", err)
			return "", err
		}
	}
	if post.Id != "" && *res.ID != "" {
		p.API.KVSet("mattermost_teams_"+post.Id, []byte(*res.ID))
		p.API.KVSet("teams_mattermost_"+*res.ID, []byte(post.Id))
	}
	return *res.ID, nil
}

func (p *Plugin) clearSubscriptions() error {
	subscriptionsCt := p.msteamsAppClient.Subscriptions().Request()
	subscriptionsRes, err := subscriptionsCt.Get(p.msteamsAppClientCtx)
	if err != nil {
		p.API.LogError("subscription creation failed", "error", err)
		return err
	}
	for _, subscription := range subscriptionsRes {
		deleteSubCt := p.msteamsAppClient.Subscriptions().ID(*subscription.ID).Request()
		err := deleteSubCt.Delete(p.msteamsAppClientCtx)
		if err != nil {
			p.API.LogError("subscription creation failed", "error", err)
			return err
		}
	}
	return nil
}

func (p *Plugin) subscribeToChannel(ctx context.Context, link ChannelLink) error {
	teamId := link.MSTeamsTeam
	channelId := link.MSTeamsChannel

	resource := "teams/" + teamId + "/channels/" + channelId + "/messages"
	expirationDateTime := time.Now().Add(60 * time.Minute)
	notificationURL := "https://matterbridge-jespino.eu.ngrok.io/plugins/com.mattermost.matterbridge-plugin/"
	clientState := "secret"
	changeType := "created"
	subscription := msgraph.Subscription{
		Resource:           &resource,
		ExpirationDateTime: &expirationDateTime,
		NotificationURL:    &notificationURL,
		ClientState:        &clientState,
		ChangeType:         &changeType,
	}
	ct := p.msteamsAppClient.Subscriptions().Request()
	res, err := ct.Add(p.msteamsAppClientCtx, &subscription)
	if err != nil {
		p.API.LogError("subscription creation failed", "error", err)
		return err
	}
	p.API.LogError("subscription created", "subscription", *res.ID)

	p.subscriptionsToLinksMutex.Lock()
	p.subscriptionsToLinks[*res.ID] = link
	p.subscriptionsToLinksMutex.Unlock()

	for {
		select {
		case <-time.After(time.Minute):
			expirationDateTime := time.Now().Add(10 * time.Minute)
			updatedSubscription := msgraph.Subscription{
				ExpirationDateTime: &expirationDateTime,
			}
			ct := p.msteamsAppClient.Subscriptions().ID(*res.ID).Request()
			err := ct.Update(p.msteamsAppClientCtx, &updatedSubscription)
			if err != nil {
				p.API.LogError("error updating subscription", "error", err)
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}
