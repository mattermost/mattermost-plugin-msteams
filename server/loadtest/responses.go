package loadtest

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost/server/public/model"
)

func initApplications(url string) (*http.Response, error) {
	r := regexp.MustCompile(`/v1.0/applications\(appId='(.+)'\)`)
	result := r.FindSubmatch([]byte(url))
	if len(result) > 1 {
		applicationId = string(result[1])
		log("Init Application", "id", applicationId)
		return NewJsonResponse(200, map[string]any{
			"@odata.context": "https://graph.microsoft.com/v1.0/$metadata#applications/$entity",
			"id":             applicationId,
			"passwordCredentials": []any{
				map[string]any{
					"@odata.type":   "microsoft.graph.passwordCredential",
					"displayName":   "Load Test",
					"endDateTime":   time.Now().Add(24 * 30 * time.Hour).Format(time.RFC3339),
					"hint":          "Use for load test",
					"keyId":         uuid.New().String(),
					"startDateTime": time.Now().Format(time.RFC3339),
				},
			},
		})
	}

	return NewJsonResponse(403, nil)
}

func initDiscoverInstance() (*http.Response, error) {
	log("for Discovery")

	return NewJsonResponse(200, map[string]any{
		"tenant_discovery_endpoint": "https://login.microsoftonline.com/" + strings.ToLower(TenantId) + "/v2.0/.well-known/openid-configuration",
		"api-version":               "1.1",
		"metadata": []map[string]any{
			{"preferred_network": "login.microsoftonline.com", "preferred_cache": "login.windows.net", "aliases": []string{"login.microsoftonline.com", "login.windows.net", "login.microsoft.com", "sts.windows.net"}},
			{"preferred_network": "login.partner.microsoftonline.cn", "preferred_cache": "login.partner.microsoftonline.cn", "aliases": []string{"login.partner.microsoftonline.cn", "login.chinacloudapi.cn"}},
			{"preferred_network": "login.microsoftonline.de", "preferred_cache": "login.microsoftonline.de", "aliases": []string{"login.microsoftonline.de"}},
			{"preferred_network": "login.microsoftonline.us", "preferred_cache": "login.microsoftonline.us", "aliases": []string{"login.microsoftonline.us", "login.usgovcloudapi.net"}},
			{"preferred_network": "login-us.microsoftonline.com", "preferred_cache": "login-us.microsoftonline.com", "aliases": []string{"login-us.microsoftonline.com"}},
		},
	})
}

func initOpenIdConfigure() (*http.Response, error) {
	log("OpenId configure")

	return NewJsonResponse(200, map[string]any{
		"token_endpoint":                        "https://login.microsoftonline.com/" + strings.ToLower(TenantId) + "/oauth2/v2.0/token",
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "private_key_jwt", "client_secret_basic"},
		"jwks_uri":                              "https://login.microsoftonline.com/" + strings.ToLower(TenantId) + "/discovery/v2.0/keys",
		"response_modes_supported":              []string{"query", "fragment", "form_post"},
		"subject_types_supported":               []string{"pairwise"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"response_types_supported":              []string{"code", "id_token", "code id_token", "id_token token"},
		"scopes_supported":                      []string{"openid", "profile", "email", "offline_access"},
		"issuer":                                "https://login.microsoftonline.com/" + strings.ToLower(TenantId) + "/v2.0",
		"request_uri_parameter_supported":       false,
		"userinfo_endpoint":                     "https://graph.microsoft.com/oidc/userinfo",
		"authorization_endpoint":                "https://login.microsoftonline.com/" + strings.ToLower(TenantId) + "/oauth2/v2.0/authorize",
		"device_authorization_endpoint":         "https://login.microsoftonline.com/" + strings.ToLower(TenantId) + "/oauth2/v2.0/devicecode",
		"http_logout_supported":                 true,
		"frontchannel_logout_supported":         true,
		"end_session_endpoint":                  "https://login.microsoftonline.com/" + strings.ToLower(TenantId) + "/oauth2/v2.0/logout",
		"claims_supported":                      []string{"sub", "iss", "cloud_instance_name", "cloud_instance_host_name", "cloud_graph_host_name", "msgraph_host", "aud", "exp", "iat", "auth_time", "acr", "nonce", "preferred_username", "name", "tid", "ver", "at_hash", "c_hash", "email"},
		"kerberos_endpoint":                     "https://login.microsoftonline.com/" + strings.ToLower(TenantId) + "/kerberos",
		"tenant_region_scope":                   "NA",
		"cloud_instance_name":                   "microsoftonline.com",
		"cloud_graph_host_name":                 "graph.windows.net",
		"msgraph_host":                          "graph.microsoft.com",
		"rbac_url":                              "https://pas.windows.net",
	})
}

func initSubsciptions() (*http.Response, error) {
	log("for Subscriptions")
	return NewJsonResponse(200, map[string]any{
		"@odata.context":            "https://graph.microsoft.com/v1.0/$metadata#subscriptions/$entity",
		"id":                        "msteams_subscriptions_id",
		"resource":                  "/test",
		"applicationId":             applicationId,
		"changeType":                "created",
		"clientState":               "secretClientValue",
		"notificationUrl":           "https://webhook.azurewebsites.net/api/send/myNotifyClient",
		"expirationDateTime":        "2036-11-20T18:23:45.9356913Z",
		"creatorId":                 "8ee44408-0679-472c-bc2a-692812af3437",
		"latestSupportedTlsVersion": "v1_2",
		"notificationContentType":   "application/json",
	})
}

func getMSTeamChannel(url string) (*http.Response, error) {
	r := regexp.MustCompile("/v1.0/teams/(.+)/channels/(.+)")
	result := r.FindSubmatch([]byte(url))
	if len(result) >= 3 {
		team := string(result[1])
		channel := string(result[2])
		log("for GetMSTeamChannel", "team", team, "channel", channel)
		return NewJsonResponse(200, map[string]any{
			"id":              channel,
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "oneOnOne",
		})
	}

	return NewJsonResponse(404, map[string]any{})
}

func postMessageToMSTeams(req *http.Request) (*http.Response, error) {
	url := req.URL.RequestURI()
	r := regexp.MustCompile(`/v1.0/chats/(.+)/messages`)
	result := r.FindSubmatch([]byte(url))
	if len(result) > 0 {
		channelId := string(result[1])
		log("MessageHasBeenPosted POST MESSAGE")
		return NewJsonResponse(200, map[string]any{
			"id":                   model.NewId(),
			"etag":                 "1616990032035",
			"messageType":          "message",
			"createdDateTime":      time.Now().Format(time.RFC3339),
			"lastModifiedDateTime": time.Now().Format(time.RFC3339),
			"importance":           "normal",
			"locale":               "en-us",
			"webUrl":               "https://teams.microsoft.com/l/message/ms-dm-id/test-message-id",
			"from": map[string]any{
				"user": map[string]any{
					"@odata.type":      "#microsoft.graph.teamworkUserIdentity",
					"id":               "ms-user-id",
					"displayName":      "ms-user-username",
					"userIdentityType": "aadUser",
					"tenantId":         "tenant-id",
				},
			},
			"body": map[string]any{
				"contentType": "text",
				"content":     "Hello World",
			},
			"channelIdentity": map[string]any{
				"channelId": channelId,
			},
		})
	}
	log("MessageHasBeenPosted POST MESSAGE FAILED")
	return NewJsonResponse(500, map[string]any{})
}

func getOrCreateMSTeamsChat(req *http.Request) (*http.Response, error) {
	if strings.ToLower(req.Method) == "post" {
		log("MessageHasBeenPosted CREATE CHANNEL")
		return NewJsonResponse(200, map[string]any{
			"id":              "ms-dm-" + model.NewId(),
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "load test channel",
			"description":     "Load Test channel",
			"membershipType":  "oneOnOne",
		})
	} else {
		url := req.URL.RequestURI()
		r := regexp.MustCompile(`/v1.0/chats/(.+)(\?)`)
		result := r.FindSubmatch([]byte(url))
		if len(result) > 0 {
			id := string(result[1])
			log("MessageHasBeenPosted GetChat", "id", id)
			return NewJsonResponse(200, map[string]any{
				"id":              id,
				"createdDateTime": time.Now().Format(time.RFC3339),
				"displayName":     "load test channel",
				"description":     "Load Test channel",
				"membershipType":  "oneOnOne",
			})
		}
	}

	return NewJsonResponse(200, map[string]any{})
}

func getOAuthToken() (*http.Response, error) {
	return NewJsonResponse(200, map[string]any{
		"token_type":   "Bearer",
		"expires_in":   3599,
		"access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
	})
}
