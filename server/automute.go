package main

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

const (
	NotifyPropAutomuted = "msteams_automute"

	// This preference tracks if we've enabled automuting for the user by muting all of their connected channels.
	PreferenceNameAutomuteEnabled = "automute_enabled"
)

func (p *Plugin) tryEnableAutomute(userID string, skipConnectedCheck, skipPrimaryPlatformCheck bool) (bool, error) {
	enabled, err := p.shouldEnableAutomuteForUser(userID, skipConnectedCheck, skipPrimaryPlatformCheck)
	if err != nil {
		return false, err
	} else if !enabled {
		return false, nil
	}

	return p.setAutomuteEnabledForUser(userID, true)
}

// shouldEnableAutomuteForUser returns true if the given user is both connected to Teams and has their primary platform
// set to Teams.
//
// skipConnectedCheck and skipPrimaryPlatformCheck can be used to skip the respective checks if the calling code has
// already performed those checks.
func (p *Plugin) shouldEnableAutomuteForUser(userID string, skipConnectedCheck, skipPrimaryPlatformCheck bool) (bool, error) {
	var connected bool
	if skipConnectedCheck {
		connected = true
	} else {
		var err error
		connected, err = p.isUserConnected(userID)
		if err != nil {
			return false, err
		}
	}

	if !connected {
		return false, nil
	}

	var teamsPrimary bool
	if skipPrimaryPlatformCheck {
		teamsPrimary = true
	} else {
		teamsPrimary = p.isUsersPrimaryPlatformTeams(userID)
	}

	if !teamsPrimary {
		return false, nil
	}

	return true, nil
}

func (p *Plugin) tryDisableAutomute(userID string) (bool, error) {
	return p.setAutomuteEnabledForUser(userID, false)
}

func (p *Plugin) setAutomuteEnabledForUser(userID string, automuteEnabled bool) (bool, error) {
	if channelsAutomuted := p.getAutomuteIsEnabledForUser(userID); channelsAutomuted == automuteEnabled {
		// We've already automuted all the users' channels, so there's nothing to do
		return false, nil
	}

	var membersToMute []*model.ChannelMemberIdentifier

	if channels, appErr := p.API.GetChannelsForTeamForUser("", userID, true); appErr != nil {
		return false, errors.Wrap(appErr, fmt.Sprintf("Unable to get channels for user %s to automute them", userID))
	} else {
		for _, channel := range channels {
			if linked, err := p.canAutomuteChannel(channel); err != nil {
				return false, err
			} else if !linked {
				continue
			}

			membersToMute = append(membersToMute, &model.ChannelMemberIdentifier{ChannelId: channel.Id, UserId: userID})
		}
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

	i := 0
	perPage := 200
	for i < len(members) {
		start := i
		end := i + perPage
		if end > len(members) {
			end = len(members)
		}

		page := members[start:end]

		if appErr := p.API.PatchChannelMembersNotifications(page, notifyProps); appErr != nil {
			return errors.Wrap(appErr, "Unable to patch notify props for automuting")
		}

		i += perPage
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

// canAutomuteChannel returns true if the channel with the given ID is either explicitly linked to a channel in
// MS Teams or if it's a DM/GM channel that is implicitly linked to MS Teams.
func (p *Plugin) canAutomuteChannelID(channelID string) (bool, error) {
	channel, appErr := p.API.GetChannel(channelID)
	if appErr != nil {
		return false, errors.Wrap(appErr, fmt.Sprintf("Unable to get channel %s to check if it's a DM/GM channel", channelID))
	}

	return p.canAutomuteChannel(channel)
}

// canAutomuteChannel returns true if the channel is either explicitly linked to a channel in MS Teams or if it's a
// DM/GM channel that is implicitly linked to MS Teams.
func (p *Plugin) canAutomuteChannel(channel *model.Channel) (bool, error) {
	// Automute all DM/GM channels
	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		return true, nil
	}

	return p.isChannelLinked(channel.Id)
}

// isChannelLinked returns true if the channel is explicitly linked to a channel in MS Teams.
func (p *Plugin) isChannelLinked(channelID string) (bool, error) {
	link, err := p.store.GetLinkByChannelID(channelID)
	if err != nil && err != sql.ErrNoRows {
		return false, errors.Wrap(err, fmt.Sprintf("Unable to determine if channel %s is linked to MS Teams", channelID))
	}

	// The channel is linked as long as a ChannelLink exists
	return link != nil, nil
}
