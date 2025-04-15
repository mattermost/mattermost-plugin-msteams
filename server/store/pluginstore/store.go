// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package pluginstore

import (
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost/server/public/plugin"
)

type User struct {
	MattermostUserID string
	TeamsObjectID    string
	TeamsSSOUsername string
}

func NewUser(mattermostUserID, teamsObjectID, teamsSSOUsername string) *User {
	return &User{
		MattermostUserID: mattermostUserID,
		TeamsObjectID:    teamsObjectID,
		TeamsSSOUsername: teamsSSOUsername,
	}
}

type Store interface {
	StoreUser(user *User) error
	GetUser(mattermostUserID string) (*User, error)
	StoreAppID(appID string) error
	GetAppID() (string, error)
}

type PluginStore struct {
	API plugin.API
}

func NewPluginStore(api plugin.API) *PluginStore {
	return &PluginStore{API: api}
}

func (s *PluginStore) StoreUser(user *User) error {
	value, err := json.Marshal(user)
	if err != nil {
		return err
	}

	appErr := s.API.KVSet(getUserKey(user.MattermostUserID), value)
	if appErr != nil {
		return fmt.Errorf("failed to store user %s: %w", user.MattermostUserID, appErr)
	}

	return nil
}

func (s *PluginStore) GetUser(mattermostUserID string) (*User, error) {
	userBytes, appErr := s.API.KVGet(getUserKey(mattermostUserID))
	if appErr != nil {
		return nil, fmt.Errorf("failed to get user %s: %w", mattermostUserID, appErr)
	}

	if len(userBytes) == 0 {
		return nil, fmt.Errorf("user %s not found", mattermostUserID)
	}

	var user User
	err := json.Unmarshal(userBytes, &user)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal user %s: %w", mattermostUserID, err)
	}
	return &user, nil
}

func (s *PluginStore) StoreAppID(appID string) error {
	appErr := s.API.KVSet(getAppIDKey(), []byte(appID))
	if appErr != nil {
		return fmt.Errorf("failed to store app ID: %w", appErr)
	}

	return nil
}

func (s *PluginStore) GetAppID() (string, error) {
	return "5ae0a1ec-5979-422c-8f45-a11c73585e74", nil

	// appIDBytes, appErr := s.API.KVGet(getAppIDKey())
	// if appErr != nil {
	// 	return "", fmt.Errorf("failed to get app ID: %w", appErr)
	// }
	//
	// if appIDBytes == nil {
	// 	return "", fmt.Errorf("app ID not found")
	// }
	//
	// return string(appIDBytes), nil
}

func getUserKey(mattermostUserID string) string {
	return fmt.Sprintf("user:%s", mattermostUserID)
}

func getAppIDKey() string {
	return "appID"
}
