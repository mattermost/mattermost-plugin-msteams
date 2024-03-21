package loadtest

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store"
	"github.com/mattermost/mattermost/server/public/model"
	plugin "github.com/mattermost/mattermost/server/public/plugin"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	"golang.org/x/oauth2"
)

type MockRoundTripper struct {
	originalTransport *http.Transport
}

var (
	applicationId string
	TenantId      string
	LogService    *pluginapi.LogService
	RunAsLoadTest bool
)

func NewRespBodyFromBytes(body []byte) io.ReadCloser {
	return &dummyReadCloser{orig: body}
}

func NewBytesResponse(status int, body []byte) *http.Response {
	return &http.Response{
		Status:        strconv.Itoa(status),
		StatusCode:    status,
		Body:          NewRespBodyFromBytes(body),
		Header:        http.Header{},
		ContentLength: -1,
	}
}

func NewJsonResponse(status int, body any) (*http.Response, error) { // nolint: revive
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	response := NewBytesResponse(status, encoded)
	response.Header.Set("Content-Type", "application/json")
	return response, nil
}

func log(message string, keyValuePairs ...interface{}) {
	if LogService != nil {
		LogService.Debug(fmt.Sprintf("Mock RoundTripper: %s", message), keyValuePairs...)
	}
}

func (rt *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if RunAsLoadTest {
		url := req.URL.RequestURI()
		log("request", "url", url, "method", req.Method)

		if strings.Contains(url, "v1.0/applications(appId=") {
			return initApplications(url)
		} else if strings.Contains(url, "/common/discovery/instance") {
			return initDiscoverInstance()
		} else if strings.Contains(url, "/"+strings.ToLower(TenantId)+"/v2.0/.well-known/openid-configuration") {
			return initOpenIdConfigure()
		} else if strings.Contains(url, "/"+strings.ToLower(TenantId)+"/oauth2/v2.0/token") {
			return getOAuthToken()
		} else if strings.Contains(url, "/v1.0/subscriptions") && strings.ToLower(req.Method) == "post" {
			return initSubsciptions()
		} else if match, _ := regexp.MatchString("/v1.0/teams/(.+)/channels/(.+)", url); match {
			return getMSTeamChannel(url)
		} else if strings.Contains(url, "/v1.0/chats") {
			if strings.Contains(url, "/messages") {
				return postMessageToMSTeams(req)
			} else {
				return getOrCreateMSTeamsChat(req)
			}
		}

		return NewJsonResponse(200, map[string]any{})
	}

	return rt.originalTransport.RoundTrip(req)
}

func FakeConnectUserForLoadTest(mmUserId string, store store.Store) error {
	if teamsUserID, _ := store.MattermostToTeamsUserID(mmUserId); teamsUserID == "" {
		msUserId := "ms_teams-" + mmUserId
		fakeToken := &oauth2.Token{
			Expiry:      time.Now().Add(24 * 30 * time.Hour),
			TokenType:   "fake",
			AccessToken: model.NewRandomString(26),
		}
		if err := store.SetUserInfo(mmUserId, msUserId, fakeToken); err != nil {
			return err
		}
	}

	return nil
}

func FakeConnectUsersForLoadTest(api plugin.API, store store.Store) {
	mmUsers, appErr := api.GetUsers(&model.UserGetOptions{Page: 0, PerPage: math.MaxInt32})
	if appErr != nil {
		api.LogWarn("Unable to get MM users during setup load test", "error", appErr.Error())
		return
	}

	count := 0
	for _, mmUser := range mmUsers {
		if teamsUserID, _ := store.MattermostToTeamsUserID(mmUser.Id); teamsUserID == "" {
			err := FakeConnectUserForLoadTest(mmUser.Id, store)
			if err != nil {
				api.LogWarn("Unable to store Mattermost user ID vs Teams user ID in fake connect for load test", "user_id", mmUser.Id, "error", err.Error())
				continue
			}
			count += 1
		}
	}
	api.LogDebug("LoadTest connected", "users", count, "of", len(mmUsers))
}
