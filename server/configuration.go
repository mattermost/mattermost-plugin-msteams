// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

// configuration captures the plugin's external configuration as exposed in the Mattermost server
// configuration, as well as values computed from the configuration. Any public fields will be
// deserialized from the Mattermost server configuration in OnConfigurationChange.
//
// As plugins are inherently concurrent (hooks being called asynchronously), and the plugin
// configuration can change at any time, access to the configuration must be synchronized. The
// strategy used in this plugin is to guard a pointer to the configuration, and clone the entire
// struct whenever it changes. You may replace this with whatever strategy you choose.
//
// If you add non-reference types to your configuration struct, be sure to rewrite Clone as a deep
// copy appropriate for your types.
type configuration struct {
	DisableSyncJob                  bool   `json:"disableSyncJob"`
	TenantID                        string `json:"tenantid"`
	ClientID                        string `json:"clientid"`
	ClientSecret                    string `json:"clientsecret"`
	EncryptionKey                   string `json:"encryptionkey"`
	EvaluationAPI                   bool   `json:"evaluationapi"`
	WebhookSecret                   string `json:"webhooksecret"`
	MaxSizeForCompleteDownload      int    `json:"maxSizeForCompleteDownload"`
	BufferSizeForFileStreaming      int    `json:"bufferSizeForFileStreaming"`
	ConnectedUsersAllowed           int    `json:"connectedUsersAllowed"`
	ConnectedUsersRestricted        bool   `json:"connectedUsersRestricted"`
	ConnectedUsersMaxPendingInvites int    `json:"connectedUsersMaxPendingInvites"`
	DisableCheckCredentials         bool   `json:"internalDisableCheckCredentials"`
	EnableUserActivityNotifications bool   `json:"enableUserActivityNotifications"`
}

func (c *configuration) ProcessConfiguration() {
	c.TenantID = strings.TrimSpace(c.TenantID)
	c.ClientID = strings.TrimSpace(c.ClientID)
	c.ClientSecret = strings.TrimSpace(c.ClientSecret)
	c.EncryptionKey = strings.TrimSpace(c.EncryptionKey)
	c.WebhookSecret = strings.TrimSpace(c.WebhookSecret)
	if c.MaxSizeForCompleteDownload < 0 {
		c.MaxSizeForCompleteDownload = 0
	}
	if c.BufferSizeForFileStreaming <= 0 {
		c.BufferSizeForFileStreaming = 20
	}
}

func (p *Plugin) validateConfiguration(configuration *configuration) error {
	configuration.ProcessConfiguration()
	if configuration.TenantID == "" {
		return errors.New("tenant ID should not be empty")
	}
	if configuration.ClientID == "" {
		return errors.New("client ID should not be empty")
	}
	if configuration.ClientSecret == "" {
		return errors.New("client secret should not be empty")
	}
	if configuration.EncryptionKey == "" {
		return errors.New("encryption key should not be empty")
	}
	if configuration.WebhookSecret == "" {
		return errors.New("webhook secret should not be empty")
	}

	return nil
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *configuration) Clone() *configuration {
	var clone = *c
	return &clone
}

func (c *configuration) ToMap() (map[string]interface{}, error) {
	var out map[string]interface{}
	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

// setConfiguration replaces the active configuration under lock.
//
// Do not call setConfiguration while holding the configurationLock, as sync.Mutex is not
// reentrant. In particular, avoid using the plugin API entirely, as this may in turn trigger a
// hook back into the plugin. If that hook attempts to acquire this lock, a deadlock may occur.
//
// This method panics if setConfiguration is called with the existing configuration. This almost
// certainly means that the configuration was modified without being cloned and may result in
// an unsafe access.
func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		// Ignore assignment if the configuration struct is empty. Go will optimize the
		// allocation for same to point at the same memory address, breaking the check
		// above.
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *Plugin) OnConfigurationChange() error {
	var configuration = new(configuration)

	// Load the public configuration fields from the Mattermost server configuration.
	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	if err := p.validateConfiguration(configuration); err != nil {
		return err
	}

	p.setConfiguration(configuration)

	// Only restart the application if the OnActivate is already executed
	if p.store != nil {
		go p.restart()
	}

	return nil
}
