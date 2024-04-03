package loadtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

var (
	subscription map[string]interface{}
)

func initApplications(url string) (*http.Response, error) {
	r := regexp.MustCompile(`/v1.0/applications\(appId='(.+)'\)`)
	result := r.FindSubmatch([]byte(url))
	if len(result) > 1 {
		applicationId := string(result[1])
		if applicationId == Settings.applicationId {
			return NewJsonResponse(200, map[string]any{
				"@odata.context": "https://graph.microsoft.com/v1.0/$metadata#applications/$entity",
				"id":             applicationId,
				"passwordCredentials": []any{
					map[string]any{
						"@odata.type":   "microsoft.graph.passwordCredential",
						"displayName":   "Load Test",
						"endDateTime":   time.Now().Add(24 * 30 * time.Hour).Format(time.RFC3339),
						"hint":          Settings.secret[:4],
						"keyId":         uuid.New().String(),
						"startDateTime": time.Now().Format(time.RFC3339),
					},
				},
			})
		}
	}

	return NewErrorResponse(500, "Mock: initApplications could not find submatch for the regex")
}

func initDiscoverInstance() (*http.Response, error) {
	return NewJsonResponse(200, map[string]any{
		"tenant_discovery_endpoint": "https://login.microsoftonline.com/" + strings.ToLower(Settings.tenantId) + "/v2.0/.well-known/openid-configuration",
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
	return NewJsonResponse(200, map[string]any{
		"token_endpoint":                        "https://login.microsoftonline.com/" + strings.ToLower(Settings.tenantId) + "/oauth2/v2.0/token",
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "private_key_jwt", "client_secret_basic"},
		"jwks_uri":                              "https://login.microsoftonline.com/" + strings.ToLower(Settings.tenantId) + "/discovery/v2.0/keys",
		"response_modes_supported":              []string{"query", "fragment", "form_post"},
		"subject_types_supported":               []string{"pairwise"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"response_types_supported":              []string{"code", "id_token", "code id_token", "id_token token"},
		"scopes_supported":                      []string{"openid", "profile", "email", "offline_access"},
		"issuer":                                "https://login.microsoftonline.com/" + strings.ToLower(Settings.tenantId) + "/v2.0",
		"request_uri_parameter_supported":       false,
		"userinfo_endpoint":                     "https://graph.microsoft.com/oidc/userinfo",
		"authorization_endpoint":                "https://login.microsoftonline.com/" + strings.ToLower(Settings.tenantId) + "/oauth2/v2.0/authorize",
		"device_authorization_endpoint":         "https://login.microsoftonline.com/" + strings.ToLower(Settings.tenantId) + "/oauth2/v2.0/devicecode",
		"http_logout_supported":                 true,
		"frontchannel_logout_supported":         true,
		"end_session_endpoint":                  "https://login.microsoftonline.com/" + strings.ToLower(Settings.tenantId) + "/oauth2/v2.0/logout",
		"claims_supported":                      []string{"sub", "iss", "cloud_instance_name", "cloud_instance_host_name", "cloud_graph_host_name", "msgraph_host", "aud", "exp", "iat", "auth_time", "acr", "nonce", "preferred_username", "name", "tid", "ver", "at_hash", "c_hash", "email"},
		"kerberos_endpoint":                     "https://login.microsoftonline.com/" + strings.ToLower(Settings.tenantId) + "/kerberos",
		"tenant_region_scope":                   "NA",
		"cloud_instance_name":                   "microsoftonline.com",
		"cloud_graph_host_name":                 "graph.windows.net",
		"msgraph_host":                          "graph.microsoft.com",
		"rbac_url":                              "https://pas.windows.net",
	})
}

func initSubsciptions() (*http.Response, error) {
	subscription = map[string]any{
		"@odata.context":            "https://graph.microsoft.com/v1.0/$metadata#subscriptions/$entity",
		"id":                        "msteams_subscriptions_id",
		"resource":                  "/test",
		"applicationId":             Settings.applicationId,
		"changeType":                "created",
		"clientState":               "secretClientValue",
		"notificationUrl":           fmt.Sprintf("%schanges", Settings.baseUrl),
		"expirationDateTime":        "2036-11-20T18:23:45.9356913Z",
		"creatorId":                 "8ee44408-0679-472c-bc2a-692812af3437",
		"latestSupportedTlsVersion": "v1_2",
		"notificationContentType":   "application/json",
	}
	return NewJsonResponse(200, subscription)
}

func getSubscriptions() (*http.Response, error) {
	if subscription != nil {
		return NewJsonResponse(200, map[string]any{
			"@odata.context": "https://graph.microsoft.com/v1.0/$metadata#subscriptions",
			"value":          []map[string]any{subscription},
		})
	}
	log("No subscriptions found!!")
	return NewJsonResponse(200, map[string]any{})
}

func getMSTeamChannel(url string) (*http.Response, error) {
	r := regexp.MustCompile("/v1.0/teams/(.+)/channels/(.+)")
	result := r.FindSubmatch([]byte(url))
	if len(result) >= 3 {
		team := string(result[1])
		channel := string(result[2])
		log("getMSTeamChannel", "team", team, "channel", channel)
		return NewJsonResponse(200, map[string]any{
			"id":              channel,
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "test channel",
			"description":     "Test channel",
			"membershipType":  "oneOnOne",
		})
	}

	return NewErrorResponse(500, "Mock: getMSTeamChannel could not find submatch for the regex")
}

func postMessageToMSTeams(req *http.Request) (*http.Response, error) {
	reqUrl := req.URL.RequestURI()
	r := regexp.MustCompile(`/v1.0/chats/(.+)/messages`)
	result := r.FindSubmatch([]byte(reqUrl))
	content := getPostContentAsMD(req)

	if len(result) > 0 {
		_, msUserId := getUserDataFromAuthHeader(req)
		channelId := string(result[1])
		id := model.NewId()

		if Settings.maxIncomingPosts > 0 {
			simulatePostsToChat(channelId, msUserId, content)
		}

		return NewJsonResponse(201, map[string]any{
			"id":                   id,
			"etag":                 id,
			"messageType":          "message",
			"createdDateTime":      time.Now().Format(time.RFC3339),
			"lastModifiedDateTime": time.Now().Format(time.RFC3339),
			"importance":           "normal",
			"locale":               "en-us",
			"webUrl":               fmt.Sprintf("https://teams.microsoft.com/l/message/%s/%s", url.QueryEscape(channelId), url.QueryEscape(id)),
			"from": map[string]any{
				"user": map[string]any{
					"@odata.type":      "#microsoft.graph.teamworkUserIdentity",
					"id":               msUserId,
					"displayName":      msUserId,
					"userIdentityType": "aadUser",
					"tenantId":         Settings.tenantId,
				},
			},
			"body": map[string]any{
				"contentType": "text",
				"content":     getHtmlFromMD(content),
			},
			"channelIdentity": map[string]any{
				"channelId": channelId,
			},
		})
	}
	return NewErrorResponse(500, "Mock: postMessageToMSTeams could not find submatch for the regex")
}

func getChatMessage(reqUrl string) (*http.Response, error) {
	r := regexp.MustCompile(`/v1.0/chats/(.+)/messages/(.+)`)
	result := r.FindSubmatch([]byte(reqUrl))
	if len(result) == 3 {
		chatId := string(result[1])
		msgId := string(result[2])
		text := "This is an incoming message"

		decodedValue, err := url.QueryUnescape(msgId)
		if err != nil {
			return NewErrorResponse(500, fmt.Sprintf("Mock: getChatMessage %s", err.Error()))
		}

		re := regexp.MustCompile(`(.+){{{((.|\n|\r|\t)*?)}}}`)
		msg := re.FindSubmatch([]byte(decodedValue))
		if len(msg) > 0 {
			msgId = string(msg[1])
			text = string(msg[2])
		}

		otherUserId := ""
		if strings.HasPrefix(chatId, "ms-dm-") {
			otherUserId = "ms_teams-" + getOtherUserFromChannelId(chatId, "")
		} else if strings.HasPrefix(chatId, "ms-gm-") {
			otherUserId = "ms_teams-" + getRandomUserFromChannelId(chatId, "")
		}

		if otherUserId != "" {
			content := buildMessageContent(chatId, msgId, text, otherUserId)
			return NewJsonResponse(200, content)
		}
	}

	return NewErrorResponse(500, "Mock: getChatMessage could not find submatch for the regex")
}

func getOrCreateMSTeamsChat(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodPost {
		uncompressedBody, err := uncompressRequestBody(req)
		if err != nil {
			return NewErrorResponse(500, fmt.Sprintf("Mock: getOrCreateMSTeamsChat %s", err.Error()))
		}

		chat := struct {
			ChatType string
			Members  []struct {
				Type     string `json:"@odata.type"`
				UserBind string `json:"user@odata.bind"`
				Roles    []string
			}
		}{}

		err = json.Unmarshal(uncompressedBody, &chat)
		if err != nil {
			return NewErrorResponse(500, fmt.Sprintf("Mock: getOrCreateMSTeamsChat %s", err.Error()))
		}
		members := []string{}
		r := regexp.MustCompile(`https:\/\/graph.microsoft.com\/v1.0\/users\('(.+)'\)`)
		for _, member := range chat.Members {
			result := r.FindSubmatch([]byte(member.UserBind))
			if len(result) > 0 {
				members = append(members, strings.Replace(string(result[1]), "ms_teams-", "", 1))
			}
		}

		id := ""
		chatType := ""
		if len(members) == 2 {
			id = "ms-dm-" + model.GetDMNameFromIds(members[0], members[1])
			chatType = models.ONEONONE_CHATTYPE.String()
		} else {
			id = "ms-gm-" + getGMNameFromIds(members)
			chatType = models.GROUP_CHATTYPE.String()
		}

		return NewJsonResponse(201, map[string]any{
			"id":              id,
			"createdDateTime": time.Now().Format(time.RFC3339),
			"displayName":     "load test channel",
			"description":     "Load Test channel",
			"chatType":        chatType,
		})
	} else {
		url := req.URL.RequestURI()
		r := regexp.MustCompile(`/v1.0/chats/(.+)(\?)`)
		result := r.FindSubmatch([]byte(url))
		if len(result) > 0 {
			id := string(result[1])
			members := []map[string]any{}
			var userIds []string
			chatType := ""
			isDM := strings.HasPrefix(id, "ms-dm-")
			isGM := strings.HasPrefix(id, "ms-gm-")

			if isDM {
				chatType = models.ONEONONE_CHATTYPE.String()
				userIds = strings.Split(strings.Replace(id, "ms-dm-", "", 1), "__")
			} else if isGM {
				chatType = models.GROUP_CHATTYPE.String()
				userIds = strings.Split(strings.Replace(id, "ms-gm-", "", 1), "__")
			}

			if isDM || isGM {

				for _, uId := range userIds {
					members = append(members, map[string]any{
						"@odata.type": "#microsoft.graph.aadUserConversationMember",
						"userId":      fmt.Sprintf("ms_teams-%s", uId),
					})
				}

				return NewJsonResponse(200, map[string]any{
					"id":                  id,
					"lastUpdatedDateTime": time.Now().Format(time.RFC3339),
					"displayName":         "load test channel",
					"description":         "Load Test channel",
					"chatType":            chatType,
					"members":             members,
				})
			}
		}
	}

	return NewErrorResponse(500, "Mock: getOrCreateMSTeamsChat could not find submatch for the regex")
}

func getOAuthToken() (*http.Response, error) {
	return NewJsonResponse(200, map[string]any{
		"token_type":   "Bearer",
		"expires_in":   3599,
		"access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
	})
}

func getUser(url string) (*http.Response, error) {
	r := regexp.MustCompile(`/v1.0/users/(.+)\?\$select=displayName,id,mail,userPrincipalName,userType`)
	result := r.FindSubmatch([]byte(url))
	if len(result) > 0 {
		id := string(result[1])
		mmUserId, err := Settings.store.TeamsToMattermostUserID(id)
		if err != nil {
			log("getUser failed", "error", err)
			return nil, err
		}
		user, appErr := Settings.api.GetUser(mmUserId)
		if appErr != nil {
			log("getUser failed", "error", appErr)
			return nil, err
		}

		return NewJsonResponse(200, map[string]any{
			"id":                id,
			"displayName":       fmt.Sprintf("%s %s", user.FirstName, user.LastName),
			"mail":              user.Email,
			"userPrincipalName": user.Email,
			"userType":          "User",
			"isAccountEnabled":  true,
		})
	}

	return NewErrorResponse(500, "Mock: getUser could not find submatch for the regex")
}
