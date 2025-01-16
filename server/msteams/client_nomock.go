// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

//go:build !msteamsMock
// +build !msteamsMock

package msteams

import (
	"net/http"

	khttp "github.com/microsoft/kiota-http-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
)

func getAuthClient() *http.Client {
	return http.DefaultClient
}

func getHTTPClient() *http.Client {
	defaultClientOptions := msgraphsdk.GetDefaultClientOptions()
	defaultMiddleWare := msgraphcore.GetDefaultMiddlewaresWithOptions(&defaultClientOptions)

	return khttp.GetDefaultClient(defaultMiddleWare...)
}
