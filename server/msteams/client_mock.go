// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

//go:build msteamsMock

package msteams

import (
	"crypto/tls"
	"net/http"
	"net/url"

	khttp "github.com/microsoft/kiota-http-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
)

func getAuthClient() *http.Client {
	proxyAddress := "http://mockserver:1080"
	proxyUrl, _ := url.Parse(proxyAddress)
	// Setup proxy for the token credential from azidentity
	authClient := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyUrl),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	return authClient
}

func getHTTPClient() *http.Client {
	proxyAddress := "http://mockserver:1080"
	proxyUrl, _ := url.Parse(proxyAddress)

	// Get default middleware from SDK
	defaultClientOptions := msgraphsdk.GetDefaultClientOptions()
	defaultMiddleWare := msgraphcore.GetDefaultMiddlewaresWithOptions(&defaultClientOptions)

	transport := khttp.NewCustomTransportWithParentTransport(&http.Transport{
		Proxy:           http.ProxyURL(proxyUrl),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}, defaultMiddleWare...)

	// Create an HTTP client with the middleware
	httpClient, _ := khttp.GetClientWithProxySettings(proxyAddress, defaultMiddleWare...)
	httpClient.Transport = transport
	return httpClient
}
