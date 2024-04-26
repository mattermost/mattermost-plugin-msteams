package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

const (
	NotifyPropAutomuted = "msteams_automute"

	// This preference tracks if we've enabled automuting for the user by muting all of their connected channels.
	PreferenceNameAutomuteEnabled = "automute_enabled"
)

// enableAutomute mutes all of the user's linked channels and DMs and sets a preference to track that automute is
// enabled. It assumes that the caller has checked that the user is both connected and has their primary platform
// set to MS Teams.
func (p *Plugin) enableAutomute(userID string) (bool, error) {
	return p.setAutomuteEnabledForUser(userID, true)
}

// disableAutomute unmutes all of the user's linked channels and DMs and sets a preference to track that automute is
// disabled. It assumes that the user is either not connected to MS Teams or has their primary platform set to MM.
func (p *Plugin) disableAutomute(userID string) (bool, error) {
	return p.setAutomuteEnabledForUser(userID, false)
}

func (p *Plugin) setAutomuteEnabledForUser(userID string, automuteEnabled bool) (bool, error) {
	if channelsAutomuted := p.getAutomuteIsEnabledForUser(userID); channelsAutomuted == automuteEnabled {
		// We've already automuted all the users' channels, so there's nothing to do
		return false, nil
	}

	var membersToMute []*model.ChannelMemberIdentifier

	channels, appErr := p.API.GetChannelsForTeamForUser("", userID, true)
	if appErr != nil {
		return false, errors.Wrap(appErr, fmt.Sprintf("Unable to get channels for user %s to automute them", userID))
	}

	for _, channel := range channels {
		if linked, err := p.canAutomuteChannel(channel); err != nil {
			return false, err
		} else if !linked {
			continue
		}

		membersToMute = append(membersToMute, &model.ChannelMemberIdentifier{ChannelId: channel.Id, UserId: userID})
	}

	if err := p.setChannelMembersAutomuted(membersToMute, automuteEnabled); err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("Unable to automute all channels for linked user %s", userID))
	}

	if err := p.setAutomuteIsEnabledForUser(userID, automuteEnabled); err != nil {
		return false, err
	}

	return true, nil
}

func (p *Plugin) setChannelMembersAutomuted(members []*model.ChannelMemberIdentifier, automuteEnabled bool) error {
	notifyProps := getNotifyPropsForAutomute(automuteEnabled)

	perPage := 200
	for i := 0; i < len(members); i += perPage {
		start := i
		end := i + perPage
		if end > len(members) {
			end = len(members)
		}

		page := members[start:end]

		if appErr := p.API.PatchChannelMembersNotifications(page, notifyProps); appErr != nil {
			return errors.Wrap(appErr, "Unable to patch notify props for automuting")
		}
	}

	return nil
}

func getNotifyPropsForAutomute(automuteEnabled bool) map[string]string {
	if automuteEnabled {
		return map[string]string{
			model.MarkUnreadNotifyProp: model.ChannelMarkUnreadMention,
			NotifyPropAutomuted:        "true",
		}
	}

	return map[string]string{
		model.MarkUnreadNotifyProp: model.ChannelMarkUnreadAll,
		NotifyPropAutomuted:        "false",
	}
}

// getAutomuteIsEnabledForUser returns true if we've muted all of the user's linked channels.
func (p *Plugin) getAutomuteIsEnabledForUser(userID string) bool {
	pref, appErr := p.API.GetPreferenceForUser(userID, PreferenceCategoryPlugin, PreferenceNameAutomuteEnabled)
	if appErr != nil {
		// Default to false if no preference is found
		return false
	}

	return pref.Value == "true"
}

// setAutomuteIsEnabledForUser sets a preference to track if we've muted all of the user's linked channels.
func (p *Plugin) setAutomuteIsEnabledForUser(userID string, channelsAutomuted bool) error {
	appErr := p.API.UpdatePreferencesForUser(userID, []model.Preference{
		{
			UserId:   userID,
			Category: PreferenceCategoryPlugin,
			Name:     PreferenceNameAutomuteEnabled,
			Value:    strconv.FormatBool(channelsAutomuted),
		},
	})
	if appErr != nil {
		return errors.Wrap(appErr, fmt.Sprintf("Unable to set preference to track that channels are automuted for user %s", userID))
	}

	return nil
}

func (p *Plugin) isUsersPrimaryPlatformTeams(userID string) bool {
	pref, appErr := p.API.GetPreferenceForUser(userID, PreferenceCategoryPlugin, PreferenceNamePlatform)
	if appErr != nil {
		// GetPreferenceForUser returns an error when a preference is unset, so we default to MM being primary platform
		return false
	}

	return pref.Value == PreferenceValuePlatformMSTeams
}

func (p *Plugin) isUserConnected(userID string) (bool, error) {
	token, err := p.store.GetTokenForMattermostUser(userID)
	if err != nil && err != sql.ErrNoRows {
		return false, errors.Wrap(err, "Unable to determine if user is connected to MS Teams")
	}

	return token != nil, nil
}

// canAutomuteChannelID returns true if the channel is either explicitly linked to a channel in MS Teams or if it's a
// DM/GM channel that is implicitly linked to MS Teams.
func (p *Plugin) canAutomuteChannelID(channelID string) (bool, error) {
	channel, err := p.API.GetChannel(channelID)
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("Unable to get channel %s to determine if it can be automuted", channelID))
	}

	return p.canAutomuteChannel(channel)
}

// canAutomuteChannel returns true if the channel is either explicitly linked to a channel in MS Teams or if it's a
// DM/GM channel that is implicitly linked to MS Teams.
func (p *Plugin) canAutomuteChannel(channel *model.Channel) (bool, error) {
	// Automute all GM channels
	if channel.Type == model.ChannelTypeGroup {
		return true, nil
	} else if channel.Type == model.ChannelTypeDirect {
		userIDs := strings.Split(channel.Name, "__")
		for _, userID := range userIDs {
			user, appErr := p.API.GetUser(userID)
			if appErr != nil {
				return false, errors.Wrap(appErr, fmt.Sprintf("Unable to get user for channel member %s ", userID))
			}
			if user.IsBot || user.IsGuest() {
				return false, nil
			}
		}
		return true, nil
	}

	link, err := p.store.GetLinkByChannelID(channel.Id)
	if err != nil && err != sql.ErrNoRows {
		return false, errors.Wrap(err, fmt.Sprintf("Unable to determine if channel %s is linked to MS Teams", channel.Id))
	}

	// The channel is linked as long as a ChannelLink exists
	return link != nil, nil
}
