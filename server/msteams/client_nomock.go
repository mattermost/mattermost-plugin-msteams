//go:build !msteamsMock
// +build !msteamsMock

package msteams

import (
	"net/http"

	loadtest "github.com/mattermost/mattermost-plugin-msteams/server/loadtest"
	khttp "github.com/microsoft/kiota-http-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
)

func getAuthClient() *http.Client {
	if loadtest.Settings != nil && loadtest.Settings.Enabled {
		return getHTTPClient()
	}
	return http.DefaultClient
}

func getHTTPClient() *http.Client {
	defaultClientOptions := msgraphsdk.GetDefaultClientOptions()
	defaultMiddleWare := msgraphcore.GetDefaultMiddlewaresWithOptions(&defaultClientOptions)

	httpClient := khttp.GetDefaultClient(defaultMiddleWare...)
	if loadtest.Settings != nil && loadtest.Settings.Enabled {
		transport := khttp.NewCustomTransportWithParentTransport(&loadtest.MockRoundTripper{}, defaultMiddleWare...)
		httpClient.Transport = transport
	}

	return httpClient
}
