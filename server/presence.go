// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
)

const (
	PresenceActivityAvailable               = "Available"
	PresenceActivityAway                    = "Away"
	PresenceActivityBeRightBack             = "BeRightBack"
	PresenceActivityBusy                    = "Busy"
	PresenceActivityDoNotDisturb            = "DoNotDisturb"
	PresenceActivityInACall                 = "InACall"
	PresenceActivityInAConferenceCall       = "InAConferenceCall"
	PresenceActivityInactive                = "Inactive"
	PresenceActivityInAMeeting              = "InAMeeting"
	PresenceActivityOffline                 = "Offline"
	PresenceActivityOffWork                 = "OffWork"
	PresenceActivityOutOfOffice             = "OutOfOffice"
	PresenceActivityPresenceUnknown         = "PresenceUnknown"
	PresenceActivityPresenting              = "Presenting"
	PresenceActivityUrgentInterruptionsOnly = "UrgentInterruptionsOnly"

	PresenceAvailabilityAvailable       = "Available"
	PresenceAvailabilityAvailableIdle   = "AvailableIdle"
	PresenceAvailabilityAway            = "Away"
	PresenceAvailabilityBeRightBack     = "BeRightBack"
	PresenceAvailabilityBusy            = "Busy"
	PresenceAvailabilityBusyIdle        = "BusyIdle"
	PresenceAvailabilityDoNotDisturb    = "DoNotDisturb"
	PresenceAvailabilityOffline         = "Offline"
	PresenceAvailabilityPresenceUnknown = "PresenceUnknown"
)

// userPresenceIsActive returns true if the user is considered online in Teams.
func userPresenceIsActive(presence clientmodels.Presence) bool {
	// If we're missing presence, default to the user being inactive.
	if presence.UserID == "" {
		return false
	}

	// Explicitly handle known activity states for being inactive or away.
	switch presence.Activity {
	case PresenceActivityOffline, PresenceActivityOffWork, PresenceActivityInactive, PresenceActivityAway:
		return false
	}

	// Otherwise, assume the user is online.
	return true
}
