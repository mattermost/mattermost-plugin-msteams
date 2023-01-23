package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/42wim/matterbridge/bridge"
	"github.com/42wim/matterbridge/bridge/config"
	"github.com/42wim/matterbridge/gateway"
	"github.com/42wim/matterbridge/gateway/bridgemap"
	prefixed "github.com/matterbridge/logrus-prefixed-formatter"
	"github.com/sirupsen/logrus"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	matterbridgeRouter *gateway.Router
	matterbridgeConfig config.Config

	userID    string
	connected bool

	clusterMutex *Mutex

	starting sync.Mutex
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
}

func (p *Plugin) start() error {
	p.starting.Lock()
	defer p.starting.Unlock()

	if p.matterbridgeRouter != nil {
		return nil
	}

	p.clusterMutex.Lock()

	logger := &logrus.Logger{
		Out: os.Stdout,
		Formatter: &prefixed.TextFormatter{
			PrefixPadding: 13,
			DisableColors: true,
		},
		Level: logrus.InfoLevel,
	}

	p.matterbridgeConfig = config.NewConfigFromString(logger, []byte(p.configuration.Config))

	var err error
	p.matterbridgeRouter, err = gateway.NewRouter(logger, p.matterbridgeConfig, bridgemap.FullMap)
	if err != nil {
		return err
	}

	if err = p.matterbridgeRouter.Start(); err != nil {
		return err
	}
	p.connected = true

	go func() {
		for msg := range p.matterbridgeRouter.MattermostPlugin {
			if err != nil {
				p.API.LogError("Error processing message: unable to get the user")
				continue
			}
			splittedName := strings.Split(msg.Channel, ":")
			if len(splittedName) != 2 {
				p.API.LogError("Error processing message: unable get the team/channel name")
				continue
			}
			teamName := splittedName[0]
			channelName := splittedName[1]
			channel, _ := p.API.GetChannelByNameForTeamName(teamName, channelName, false)
			props := make(map[string]interface{})
			props["matterbridge_"+p.userID] = true
			post := &model.Post{UserId: p.userID, ChannelId: channel.Id, Message: msg.Username + msg.Text, Props: props}
			p.API.CreatePost(post)
		}
	}()
	return nil
}

func (p *Plugin) stop() {
	if p == nil || p.matterbridgeRouter == nil {
		return
	}
	m := make(map[string]*bridge.Bridge)
	for _, gw := range p.matterbridgeRouter.Gateways {
		for _, br := range gw.Bridges {
			m[br.Account] = br
		}
	}
	for _, br := range m {
		br.Disconnect()
	}
	close(p.matterbridgeRouter.MattermostPlugin)
	close(p.matterbridgeRouter.Message)
	p.matterbridgeRouter = nil
	p.clusterMutex.Unlock()
}

func (p *Plugin) restart() error {
	p.stop()
	return p.start()
}

func (p *Plugin) OnActivate() error {
	mutex, err := NewMutex(p.API, "matterbridge-cluster-mutex")
	if err != nil {
		return err
	}
	p.clusterMutex = mutex

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

	go p.start()
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
	msg := config.Message{Username: u.Username, UserID: post.UserId, Channel: team.Name + ":" + channel.Name, Text: post.Message, ID: post.Id, Account: "mattermost.plugin", Protocol: "mattermost", Gateway: "plugin"}
	if p.connected {
		p.matterbridgeRouter.Message <- msg
	} else {
		data, err := json.Marshal(msg)
		if err != nil {
			p.API.LogError("Error processing message: unable to generate cluster message")
		}
		event := model.PluginClusterEvent{
			Id:   post.Id,
			Data: data,
		}
		if err := p.API.PublishPluginClusterEvent(event, model.PluginClusterEventSendOptions{}); err != nil {
			p.API.LogError("Error processing message: unable to deliver cluster message")
		}
	}
}

func (p *Plugin) OnPluginClusterEvent(c *plugin.Context, ev model.PluginClusterEvent) {
	if p.connected {
		var msg config.Message
		err := json.Unmarshal(ev.Data, &msg)
		if err != nil {
			p.API.LogError("Error processing message: unable to unmarshal cluster message")
		}
		p.matterbridgeRouter.Message <- msg
	}
}

func (p *Plugin) OnDeactivate() error {
	p.stop()
	return nil
}

// See https://developers.mattermost.com/extend/plugins/server/reference/
