package main

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func (p *Plugin) UserHasJoinedChannel(c *plugin.Context, channelMember *model.ChannelMember, actor *model.User) {
	// TODO MM-56498
}

func (p *Plugin) ChannelHasBeenCreated(c *plugin.Context, channel *model.Channel) {
	// TODO MM-56499
}
