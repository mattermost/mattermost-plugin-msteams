package containere2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type MockClient struct {
	api string
}

func NewMockClient(api string) (*MockClient, error) {
	mock := &MockClient{api: api}
	if err := mock.init(); err != nil {
		return nil, err
	}
	return mock, nil
}

func (m *MockClient) init() error {
	err := m.Get("/common/discovery/instance", map[string]any{
		"tenant_discovery_endpoint": "https://login.microsoftonline.com/d2888234-d303-4c94-8f45-c7348f089048/v2.0/.well-known/openid-configuration",
		"api-version":               "1.1",
		"metadata": []map[string]any{
			{"preferred_network": "login.microsoftonline.com", "preferred_cache": "login.windows.net", "aliases": []string{"login.microsoftonline.com", "login.windows.net", "login.microsoft.com", "sts.windows.net"}},
			{"preferred_network": "login.partner.microsoftonline.cn", "preferred_cache": "login.partner.microsoftonline.cn", "aliases": []string{"login.partner.microsoftonline.cn", "login.chinacloudapi.cn"}},
			{"preferred_network": "login.microsoftonline.de", "preferred_cache": "login.microsoftonline.de", "aliases": []string{"login.microsoftonline.de"}},
			{"preferred_network": "login.microsoftonline.us", "preferred_cache": "login.microsoftonline.us", "aliases": []string{"login.microsoftonline.us", "login.usgovcloudapi.net"}},
			{"preferred_network": "login-us.microsoftonline.com", "preferred_cache": "login-us.microsoftonline.com", "aliases": []string{"login-us.microsoftonline.com"}},
		},
	})
	if err != nil {
		return err
	}

	err = m.Get("/tenant-id/v2.0/.well-known/openid-configuration", map[string]any{
		"token_endpoint":                        "https://login.microsoftonline.com/d2888234-d303-4c94-8f45-c7348f089048/oauth2/v2.0/token",
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "private_key_jwt", "client_secret_basic"},
		"jwks_uri":                              "https://login.microsoftonline.com/d2888234-d303-4c94-8f45-c7348f089048/discovery/v2.0/keys",
		"response_modes_supported":              []string{"query", "fragment", "form_post"},
		"subject_types_supported":               []string{"pairwise"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"response_types_supported":              []string{"code", "id_token", "code id_token", "id_token token"},
		"scopes_supported":                      []string{"openid", "profile", "email", "offline_access"},
		"issuer":                                "https://login.microsoftonline.com/d2888234-d303-4c94-8f45-c7348f089048/v2.0",
		"request_uri_parameter_supported":       false,
		"userinfo_endpoint":                     "https://graph.microsoft.com/oidc/userinfo",
		"authorization_endpoint":                "https://login.microsoftonline.com/d2888234-d303-4c94-8f45-c7348f089048/oauth2/v2.0/authorize",
		"device_authorization_endpoint":         "https://login.microsoftonline.com/d2888234-d303-4c94-8f45-c7348f089048/oauth2/v2.0/devicecode",
		"http_logout_supported":                 true,
		"frontchannel_logout_supported":         true,
		"end_session_endpoint":                  "https://login.microsoftonline.com/d2888234-d303-4c94-8f45-c7348f089048/oauth2/v2.0/logout",
		"claims_supported":                      []string{"sub", "iss", "cloud_instance_name", "cloud_instance_host_name", "cloud_graph_host_name", "msgraph_host", "aud", "exp", "iat", "auth_time", "acr", "nonce", "preferred_username", "name", "tid", "ver", "at_hash", "c_hash", "email"},
		"kerberos_endpoint":                     "https://login.microsoftonline.com/d2888234-d303-4c94-8f45-c7348f089048/kerberos",
		"tenant_region_scope":                   "NA",
		"cloud_instance_name":                   "microsoftonline.com",
		"cloud_graph_host_name":                 "graph.windows.net",
		"msgraph_host":                          "graph.microsoft.com",
		"rbac_url":                              "https://pas.windows.net",
	})
	if err != nil {
		return err
	}

	err = m.Post("/d2888234-d303-4c94-8f45-c7348f089048/oauth2/v2.0/token", map[string]any{
		"token_type":   "Bearer",
		"expires_in":   3599,
		"access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
	})
	if err != nil {
		return err
	}

	err = m.Get("/v1.0/subscriptions", map[string]any{})
	if err != nil {
		return err
	}

	err = m.Post("/v1.0/subscriptions", map[string]any{
		"@odata.context":            "https://graph.microsoft.com/v1.0/$metadata#subscriptions/$entity",
		"id":                        uuid.New().String(),
		"resource":                  "/test",
		"applicationId":             "application-id",
		"changeType":                "created",
		"clientState":               "secretClientValue",
		"notificationUrl":           "https://webhook.azurewebsites.net/api/send/myNotifyClient",
		"expirationDateTime":        "2036-11-20T18:23:45.9356913Z",
		"creatorId":                 "8ee44408-0679-472c-bc2a-692812af3437",
		"latestSupportedTlsVersion": "v1_2",
		"notificationContentType":   "application/json",
	})
	if err != nil {
		return err
	}

	return nil
}

func (m *MockClient) Get(url string, body map[string]any) error {
	return m.Mock(http.MethodGet, url, http.StatusOK, body)
}

func (m *MockClient) Post(url string, body map[string]any) error {
	return m.Mock(http.MethodPost, url, http.StatusOK, body)
}

func (m *MockClient) Put(url string, body map[string]any) error {
	return m.Mock(http.MethodPut, url, http.StatusOK, body)
}

func (m *MockClient) Delete(url string, body map[string]any) error {
	return m.Mock(http.MethodDelete, url, http.StatusOK, body)
}

func (m *MockClient) MockError(method string, url string) error {
	return m.Mock(method, url, 401, map[string]any{
		"error": map[string]any{
			"code":    "badRequest",
			"message": "Test bad request",
			"innerError": map[string]any{
				"code":       "invalidRange",
				"request-id": "request-id",
				"date":       time.Now().Format(time.RFC3339),
			},
		},
	})
}

func (m *MockClient) Mock(method string, url string, statusCode int, body map[string]any) error {
	bodyData, err := json.Marshal(body)
	if err != nil {
		return err
	}
	mockExpectation := map[string]any{
		"priority": 0,
		"httpRequest": map[string]any{
			"method": method,
			"path":   url,
		},
		"httpResponse": map[string]any{
			"statusCode": statusCode,
			"body": map[string]any{
				"type":        "STRING",
				"contentType": "application/json",
				"string":      string(bodyData),
			},
		},
	}

	testMock, err := json.Marshal(mockExpectation)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", m.api+"/mockserver/expectation", bytes.NewReader(testMock))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (m *MockClient) Reset() error {
	req, err := http.NewRequest("PUT", m.api+"/mockserver/reset", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return m.init()
}
