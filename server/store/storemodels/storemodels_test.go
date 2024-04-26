package storemodels

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetStatsOptions_IsValid(t *testing.T) {
	makeStats := func(stats ...StatType) []StatType {
		return stats
	}

	testCases := []struct {
		name               string
		stats              []StatType
		remoteID           string
		preferenceCategory string
		activeUsersFrom    time.Time
		activeUsersTo      time.Time
		shouldBeValid      bool
	}{
		{
			name:          "RemoteID is required for StatsSyntheticUsers",
			stats:         makeStats(StatsSyntheticUsers),
			remoteID:      "",
			shouldBeValid: false,
		},
		{
			name:               "PreferenceCategory is required for StatsPrimaryPlatform",
			stats:              makeStats(StatsPrimaryPlatform),
			preferenceCategory: "",
			shouldBeValid:      false,
		},
		{
			name:            "ActiveUsersFrom is required for StatsActiveUsersSending",
			stats:           makeStats(StatsActiveUsersSending),
			activeUsersFrom: time.Time{},
			shouldBeValid:   false,
		},
		{
			name:            "ActiveUsersTo is required for StatsActiveUsersSending",
			stats:           makeStats(StatsActiveUsersSending),
			activeUsersFrom: time.Now(),
			activeUsersTo:   time.Time{},
			shouldBeValid:   false,
		},
		{
			name:            "ActiveUsersFrom is required for StatsActiveUsersReceiving",
			stats:           makeStats(StatsActiveUsersReceiving),
			activeUsersFrom: time.Time{},
			shouldBeValid:   false,
		},
		{
			name:            "ActiveUsersTo is required for StatsActiveUsersReceiving",
			stats:           makeStats(StatsActiveUsersReceiving),
			activeUsersFrom: time.Now(),
			activeUsersTo:   time.Time{},
			shouldBeValid:   false,
		},
		{
			name:               "IsValid returns nil when all required fields are provided",
			stats:              makeStats(),
			remoteID:           "remoteID",
			preferenceCategory: "preferenceCategory",
			activeUsersFrom:    time.Now(),
			activeUsersTo:      time.Now(),
			shouldBeValid:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			options := GetStatsOptions{
				RemoteID:           tc.remoteID,
				PreferenceCategory: tc.preferenceCategory,
				ActiveUsersFrom:    tc.activeUsersFrom,
				ActiveUsersTo:      tc.activeUsersTo,
				Stats:              tc.stats,
			}

			err := options.IsValid()
			if tc.shouldBeValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
func TestGetStatsOptions_MustGetStat(t *testing.T) {
	makeStats := func(stats ...StatType) []StatType {
		return stats
	}

	testCases := []struct {
		name       string
		stats      []StatType
		statToFind StatType
		expected   bool
	}{
		{
			name:       "Empty stats slice should return true",
			stats:      []StatType{},
			statToFind: StatsSyntheticUsers,
			expected:   true,
		},
		{
			name:       "StatType found in stats slice should return true",
			stats:      makeStats(StatsSyntheticUsers, StatsPrimaryPlatform),
			statToFind: StatsPrimaryPlatform,
			expected:   true,
		},
		{
			name:       "StatType not found in stats slice should return false",
			stats:      makeStats(StatsSyntheticUsers, StatsPrimaryPlatform),
			statToFind: StatsActiveUsersSending,
			expected:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			options := GetStatsOptions{
				Stats: tc.stats,
			}

			result := options.MustGetStat(tc.statToFind)
			assert.Equal(t, tc.expected, result)
		})
	}
}
