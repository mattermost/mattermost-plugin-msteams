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
	return getHTTPClient()
}

func getHTTPClient() *http.Client {
	defaultClientOptions := msgraphsdk.GetDefaultClientOptions()
	defaultMiddleWare := msgraphcore.GetDefaultMiddlewaresWithOptions(&defaultClientOptions)

	httpClient := khttp.GetDefaultClient(defaultMiddleWare...)

	transport := khttp.NewCustomTransportWithParentTransport(&loadtest.MockRoundTripper{}, defaultMiddleWare...)
	httpClient.Transport = transport

	return httpClient
}
