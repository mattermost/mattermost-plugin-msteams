package main

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/42wim/matterbridge/bridge/config"
	"github.com/42wim/matterbridge/gateway"
	msgraph "github.com/yaegashi/msgraph.go/beta"
	"github.com/yaegashi/msgraph.go/msauth"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

// TODO: Move this to settings somehow
const (
	msteamsTeamID    = "277ab716-6e73-4b88-bb1e-1151b8b2ebb0"
	msteamsChannelID = "19:f_1Tc7ppcOQtbauWM4eqoiB7gY1t-AUIJ3-OJJMqjhk1@thread.tacv2"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	matterbridgeRouter    *gateway.Router
	matterbridgeConfig    config.Config
	msteamsAppClientMutex sync.Mutex
	msteamsAppClient      *msgraph.GraphServiceRequestBuilder
	msteamsAppClientCtx   context.Context
	msteamsBotClientMutex sync.Mutex
	msteamsBotClient      *msgraph.GraphServiceRequestBuilder
	msteamsBotClientCtx   context.Context
	botID                 string
	userID                string
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.API.LogError("RECEIVING REQUEST")
	p.processMessage(w, r)
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
	go p.subscribeToChannel(msteamsTeamID, msteamsChannelID)
}

func (p *Plugin) stop() {
}

func (p *Plugin) restart() {
}

func (p *Plugin) OnActivate() error {
	bot, appErr := p.API.CreateBot(&model.Bot{
		Username:    "matterbridge",
		DisplayName: "MatterBridge",
		Description: "Created by the MatterBridge plugin.",
	})
	if appErr != nil {
		bot, err := p.API.GetUserByUsername("matterbridge")
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
	team, _ := p.API.GetTeam(channel.TeamId)
	u, _ := p.API.GetUser(post.UserId)
	parentID := []byte{}
	if post.RootId != "" {
		parentID, _ = p.API.KVGet("mattermost_teams_" + post.RootId)
	}

	msg := config.Message{
		Username: u.Username,
		UserID:   post.UserId,
		Channel:  team.Name + ":" + channel.Name,
		Text:     post.Message,
		ParentID: string(parentID),
		ID:       post.Id,
		Account:  "mattermost.plugin",
		Protocol: "mattermost",
		Gateway:  "plugin",
		Extra: map[string][]interface{}{
			"ParentID":   {string(parentID)},
			"OriginalID": {post.Id},
		},
	}
	// p.matterbridgeRouter.Message <- msg
	go p.Send(msg)
}

func (p *Plugin) OnDeactivate() error {
	p.stop()
	return nil
}

func (p *Plugin) Send(msg config.Message) (string, error) {
	p.API.LogDebug("\n\n\n=> Receiving message", "msg", msg)
	// TODO: Replace this with a template
	text := msg.Username + "@mattermost: " + msg.Text
	content := &msgraph.ItemBody{Content: &text}
	rmsg := &msgraph.ChatMessage{Body: content}

	var res *msgraph.ChatMessage
	if msg.ParentID != "" {
		var err error
		// TODO: Change the TEAMID and the CHANNELID for something that comes from the config somehow
		ct := p.msteamsBotClient.Teams().ID(msteamsTeamID).Channels().ID(msteamsChannelID).Messages().ID(msg.ParentID).Replies().Request()
		res, err = ct.Add(p.msteamsBotClientCtx, rmsg)
		if err != nil {
			p.API.LogError("Error creating reply", "error", err)
			return "", err
		}
	} else {
		var err error
		// TODO: Change the TEAMID and the CHANNELID for something that comes from the config somehow
		ct := p.msteamsBotClient.Teams().ID(msteamsTeamID).Channels().ID(msteamsChannelID).Messages().Request()
		res, err = ct.Add(p.msteamsBotClientCtx, rmsg)
		if err != nil {
			p.API.LogError("Error creating message", "error", err)
			return "", err
		}
	}
	if msg.ID != "" && *res.ID != "" {
		p.API.KVSet("mattermost_teams_"+msg.ID, []byte(*res.ID))
		p.API.KVSet("teams_mattermost_"+*res.ID, []byte(msg.ID))
	}
	return *res.ID, nil
}

// See https://developers.mattermost.com/extend/plugins/server/reference/

func (p *Plugin) subscribeToChannel(teamId, channelId string) error {
	// TODO: This should be done once on connect, not on every single case
	p.API.LogError("Startig the subscriptions")
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
	p.API.LogError("subscription created", "subscription", res)

	for {
		time.Sleep(time.Minute)

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
	}
}
