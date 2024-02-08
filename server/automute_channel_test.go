package main

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateAutomutingOnUserJoinedChannel(t *testing.T) {
	setup := func(t *testing.T, automuteEnabled bool) (*Plugin, *model.User, *model.Channel, *model.Channel) {
		t.Helper()

		p := newAutomuteTestPlugin(t)

		user := &model.User{Id: model.NewId()}
		mockUserConnected(p, user.Id)

		err := p.setAutomuteIsEnabledForUser(user.Id, automuteEnabled)
		require.NoError(t, err)

		linkedChannel, appErr := p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypeOpen})
		require.Nil(t, appErr)
		mockLinkedChannel(p, linkedChannel)

		unlinkedChannel, appErr := p.API.CreateChannel(&model.Channel{Id: model.NewId(), Type: model.ChannelTypeOpen})
		require.Nil(t, appErr)
		mockUnlinkedChannel(p, unlinkedChannel)

		return p, user, linkedChannel, unlinkedChannel
	}

	t.Run("when a user with automuting enabled joins a linked channel, the channel should be muted for that user", func(t *testing.T) {
		p, user, linkedChannel, _ := setup(t, true)

		_, appErr := p.API.AddUserToChannel(linkedChannel.Id, user.Id, user.Id)
		require.Nil(t, appErr)

		assert.EventuallyWithT(t, func(t *assert.CollectT) {
			member, appErr := p.API.GetChannelMember(linkedChannel.Id, user.Id)
			require.Nil(t, appErr)

			assert.Equal(t, model.ChannelMarkUnreadMention, member.NotifyProps[model.MarkUnreadNotifyProp])
		}, 1*time.Second, 10*time.Millisecond)
	})

	t.Run("when a user without automuting enabled joins a linked channel, nothing should happen", func(t *testing.T) {
		p, user, linkedChannel, _ := setup(t, false)

		_, appErr := p.API.AddUserToChannel(linkedChannel.Id, user.Id, user.Id)
		require.Nil(t, appErr)

		member, appErr := p.API.GetChannelMember(linkedChannel.Id, user.Id)
		require.Nil(t, appErr)

		assert.Equal(t, model.ChannelMarkUnreadAll, member.NotifyProps[model.MarkUnreadNotifyProp])
	})

	t.Run("when a user with automuting enabled joins a non-linked channel, nothing should happen", func(t *testing.T) {
		p, user, _, unlinkedChannel := setup(t, true)

		_, appErr := p.API.AddUserToChannel(unlinkedChannel.Id, user.Id, user.Id)
		require.Nil(t, appErr)

		member, appErr := p.API.GetChannelMember(unlinkedChannel.Id, user.Id)
		require.Nil(t, appErr)

		assert.Equal(t, model.ChannelMarkUnreadAll, member.NotifyProps[model.MarkUnreadNotifyProp])
	})

	t.Run("when a user without automuting enabled joins a non-linked channel, nothing should happen", func(t *testing.T) {
		p, user, _, unlinkedChannel := setup(t, false)

		_, appErr := p.API.AddUserToChannel(unlinkedChannel.Id, user.Id, user.Id)
		require.Nil(t, appErr)

		member, appErr := p.API.GetChannelMember(unlinkedChannel.Id, user.Id)
		require.Nil(t, appErr)

		assert.Equal(t, model.ChannelMarkUnreadAll, member.NotifyProps[model.MarkUnreadNotifyProp])
	})

	t.Run("should do nothing when an unconnected user joins a linked channel", func(t *testing.T) {
		p, _, linkedChannel, _ := setup(t, true)

		unconnectedUser := &model.User{Id: model.NewId()}
		mockUserNotConnected(p, unconnectedUser.Id)

		_, appErr := p.API.AddUserToChannel(linkedChannel.Id, unconnectedUser.Id, unconnectedUser.Id)
		require.Nil(t, appErr)

		member, appErr := p.API.GetChannelMember(linkedChannel.Id, unconnectedUser.Id)
		require.Nil(t, appErr)

		assert.Equal(t, model.ChannelMarkUnreadAll, member.NotifyProps[model.MarkUnreadNotifyProp])
	})

	t.Run("should do nothing when an unconnected user joins an unlinked channel", func(t *testing.T) {
		p, _, _, unlinkedChannel := setup(t, true)

		unconnectedUser := &model.User{Id: model.NewId()}
		mockUserNotConnected(p, unconnectedUser.Id)

		_, appErr := p.API.AddUserToChannel(unlinkedChannel.Id, unconnectedUser.Id, unconnectedUser.Id)
		require.Nil(t, appErr)

		member, appErr := p.API.GetChannelMember(unlinkedChannel.Id, unconnectedUser.Id)
		require.Nil(t, appErr)

		assert.Equal(t, model.ChannelMarkUnreadAll, member.NotifyProps[model.MarkUnreadNotifyProp])
	})

	t.Run("when a user with automuting enabled joins a linked channel, the channel should only be muted for them", func(t *testing.T) {
		p, user, linkedChannel, _ := setup(t, true)

		connectedUser := &model.User{Id: model.NewId()}
		mockUserConnected(p, connectedUser.Id)
		_, appErr := p.API.AddUserToChannel(linkedChannel.Id, connectedUser.Id, connectedUser.Id)
		require.Nil(t, appErr)

		unconnectedUser := &model.User{Id: model.NewId()}
		mockUserConnected(p, unconnectedUser.Id)
		_, appErr = p.API.AddUserToChannel(linkedChannel.Id, unconnectedUser.Id, connectedUser.Id)
		require.Nil(t, appErr)

		_, appErr = p.API.AddUserToChannel(linkedChannel.Id, user.Id, user.Id)
		require.Nil(t, appErr)

		time.Sleep(1 * time.Second)

		connectedMember, appErr := p.API.GetChannelMember(linkedChannel.Id, connectedUser.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelMarkUnreadAll, connectedMember.NotifyProps[model.MarkUnreadNotifyProp])

		unconnectedMember, appErr := p.API.GetChannelMember(linkedChannel.Id, unconnectedUser.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelMarkUnreadAll, unconnectedMember.NotifyProps[model.MarkUnreadNotifyProp])
	})
}

func TestUpdateAutomutingOnChannelCreated(t *testing.T) {
	t.Run("when a DM is created, should mute it for users with automuting enabled", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		channel := &model.Channel{
			Id:   model.NewId(),
			Type: model.ChannelTypeDirect,
		}

		connectedUser := &model.User{Id: model.NewId()}
		mockUserConnected(p, connectedUser.Id)
		unconnectedUser := &model.User{Id: model.NewId()}
		mockUserNotConnected(p, unconnectedUser.Id)

		err := p.setAutomuteIsEnabledForUser(connectedUser.Id, true)
		require.NoError(t, err)

		// Add channel members manually first because they'll be channel members before ChannelHasBeenCreated is called
		mockAPI := p.API.(*AutomuteAPIMock)
		mockAPI.addMockChannelMember(channel.Id, connectedUser.Id)
		mockAPI.addMockChannelMember(channel.Id, unconnectedUser.Id)

		p.ChannelHasBeenCreated(&plugin.Context{}, channel)

		connectedMember, appErr := p.API.GetChannelMember(channel.Id, connectedUser.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelMarkUnreadMention, connectedMember.NotifyProps[model.MarkUnreadNotifyProp])

		unconnectedMember, appErr := p.API.GetChannelMember(channel.Id, unconnectedUser.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelMarkUnreadAll, unconnectedMember.NotifyProps[model.MarkUnreadNotifyProp])
	})

	t.Run("when a GM is created, should mute it for users with automuting enabled", func(t *testing.T) {
		t.Skip("not working due to MM-56776")

		p := newAutomuteTestPlugin(t)

		channel := &model.Channel{
			Id:   model.NewId(),
			Type: model.ChannelTypeGroup,
		}

		connectedUserWithAutomute := &model.User{Id: model.NewId()}
		mockUserConnected(p, connectedUserWithAutomute.Id)
		connectedUserWithoutAutomute := &model.User{Id: model.NewId()}
		mockUserConnected(p, connectedUserWithoutAutomute.Id)
		unconnectedUser := &model.User{Id: model.NewId()}
		mockUserNotConnected(p, unconnectedUser.Id)

		err := p.setAutomuteIsEnabledForUser(connectedUserWithAutomute.Id, true)
		require.NoError(t, err)

		// Add channel members manually first because they'll be channel members before ChannelHasBeenCreated is called
		mockAPI := p.API.(*AutomuteAPIMock)
		mockAPI.addMockChannelMember(channel.Id, connectedUserWithAutomute.Id)
		mockAPI.addMockChannelMember(channel.Id, connectedUserWithoutAutomute.Id)
		mockAPI.addMockChannelMember(channel.Id, unconnectedUser.Id)

		p.ChannelHasBeenCreated(&plugin.Context{}, channel)

		connectedMemberWithAutomute, appErr := p.API.GetChannelMember(channel.Id, connectedUserWithAutomute.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelMarkUnreadMention, connectedMemberWithAutomute.NotifyProps[model.MarkUnreadNotifyProp])

		connectedMemberWithoutAutomute, appErr := p.API.GetChannelMember(channel.Id, connectedUserWithoutAutomute.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelMarkUnreadAll, connectedMemberWithoutAutomute.NotifyProps[model.MarkUnreadNotifyProp])

		unconnectedMember, appErr := p.API.GetChannelMember(channel.Id, unconnectedUser.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelMarkUnreadAll, unconnectedMember.NotifyProps[model.MarkUnreadNotifyProp])
	})

	t.Run("when a regular channel is created, should do nothing", func(t *testing.T) {
		p := newAutomuteTestPlugin(t)

		channel := &model.Channel{
			Id:   model.NewId(),
			Type: model.ChannelTypePrivate,
		}

		connectedUser := &model.User{Id: model.NewId()}
		mockUserConnected(p, connectedUser.Id)
		unconnectedUser := &model.User{Id: model.NewId()}
		mockUserNotConnected(p, unconnectedUser.Id)

		err := p.setAutomuteIsEnabledForUser(connectedUser.Id, true)
		require.NoError(t, err)

		// Add channel members manually first because they'll be channel members before ChannelHasBeenCreated is called
		mockAPI := p.API.(*AutomuteAPIMock)
		mockAPI.addMockChannelMember(channel.Id, connectedUser.Id)
		mockAPI.addMockChannelMember(channel.Id, unconnectedUser.Id)

		p.ChannelHasBeenCreated(&plugin.Context{}, channel)

		connectedMember, appErr := p.API.GetChannelMember(channel.Id, connectedUser.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelMarkUnreadAll, connectedMember.NotifyProps[model.MarkUnreadNotifyProp])

		unconnectedMember, appErr := p.API.GetChannelMember(channel.Id, unconnectedUser.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelMarkUnreadAll, unconnectedMember.NotifyProps[model.MarkUnreadNotifyProp])
	})
}
