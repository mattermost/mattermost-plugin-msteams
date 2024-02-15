//go:build !clientMock
// +build !clientMock

package main

import "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"

func getClientMock(_ *Plugin) *mocks.Client {
	return nil
}

func (a *API) registerClientMock() {
}
