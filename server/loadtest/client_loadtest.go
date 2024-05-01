package loadtest

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store"
	"github.com/mattermost/mattermost/server/public/model"
	plugin "github.com/mattermost/mattermost/server/public/plugin"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	"golang.org/x/oauth2"
)

type MockRoundTripper struct{}

var (
	Settings   *LoadTestSettings
	dispatcher *Dispatcher
)

func Configure(applicationId, secret, tenantId, webhookSecret, baseUrl string, enabled, simulateIncomingPosts bool, maxIncomingPosts, minIncomingPosts int, api plugin.API, store store.Store, logService *pluginapi.LogService) {
	Settings = &LoadTestSettings{
		api:                   api,
		store:                 store,
		clientId:              applicationId,
		secret:                secret,
		tenantId:              tenantId,
		baseUrl:               baseUrl,
		userTokenMap:          sync.Map{},
		Enabled:               enabled,
		simulateIncomingPosts: simulateIncomingPosts,
		maxIncomingPosts:      maxIncomingPosts,
		minIncomingPosts:      minIncomingPosts,
		logService:            logService,
		webhookSecret:         webhookSecret,
	}

	if enabled {
		SimulateQueue = make(chan PostToChatJob, 1000)
		AttachmentsSync = sync.Map{}
		Boolgen = &boolegen{src: rand.NewSource(time.Now().UnixNano())}

		dispatcher = NewDispatcher(250)
		dispatcher.Run()
	} else if dispatcher != nil {
		dispatcher.Stop()
	}
}

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

func NewErrorResponse(status int, message string) (*http.Response, error) {
	return &http.Response{
		Status:        strconv.Itoa(status),
		StatusCode:    status,
		Body:          nil,
		Header:        http.Header{},
		ContentLength: -1,
	}, fmt.Errorf(message)
}

func log(message string, keyValuePairs ...interface{}) {
	if Settings.logService != nil {
		Settings.logService.Debug(fmt.Sprintf("Mock RoundTripper: %s", message), keyValuePairs...)
	}
}

func (rt *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.RequestURI()
	log("request", "url", url, "method", req.Method)

	if strings.Contains(url, "v1.0/applications(appId=") {
		return initApplications(url)
	} else if strings.Contains(url, "/common/discovery/instance") {
		return initDiscoverInstance()
	} else if strings.Contains(url, "/"+strings.ToLower(Settings.tenantId)+"/v2.0/.well-known/openid-configuration") {
		return initOpenIdConfigure()
	} else if strings.Contains(url, "/"+strings.ToLower(Settings.tenantId)+"/oauth2/v2.0/token") {
		return getOAuthToken()
	} else if strings.Contains(url, "/v1.0/subscriptions") {
		if req.Method == http.MethodPost {
			return initSubsciptions()
		} else if req.Method == http.MethodGet {
			return getSubscriptions()
		}
	} else if match, _ := regexp.MatchString("/v1.0/teams/(.+)/channels/(.+)", url); match {
		return getMSTeamChannel(url)
	} else if strings.Contains(url, "/v1.0/chats") {
		if match, _ := regexp.MatchString(`/v1.0/chats/(.+)/messages/(.+)`, url); match {
			return getChatMessage(url)
		} else if strings.Contains(url, "/messages") {
			return postMessageToMSTeams(req)
		} else {
			return getOrCreateMSTeamsChat(req)
		}
	} else if match, _ := regexp.MatchString(`/v1.0/users/(.+)\?\$select=displayName,id,mail,userPrincipalName,userType`, url); match {
		return getUser(url)
	} else if strings.Contains(url, "/v1.0/users?$select=displayName,id,mail,userPrincipalName,userType,accountEnabled") {
		return NewJsonResponse(200, map[string]any{})
	} else if strings.Contains(url, "/v1.0/me/chats") {
		// This is called to determine to which channels the user belongs to
		// in order to avoid creating a new group chat, for the purpose of the load test
		// we will respond with no memberships so that the group chat is re-created, should have some impact in perf (minimal)
		return NewJsonResponse(200, map[string]any{})
	}

	return NewErrorResponse(404, "Mock route not implemented")
}

func FakeConnectUserForLoadTest(mmUserId string) {
	if teamsUserID, _ := Settings.store.MattermostToTeamsUserID(mmUserId); teamsUserID == "" {
		log("Connecting user to MS Teams for load test")
		msUserId := "ms_teams-" + mmUserId
		fakeToken := &oauth2.Token{
			Expiry:      time.Now().Add(24 * 30 * time.Hour),
			TokenType:   "fake",
			AccessToken: model.NewRandomString(26),
		}
		Settings.store.SetUserInfo(mmUserId, msUserId, fakeToken)
	}
}

func FakeConnectUserIfNeeded(userID string, connectedUsersAllowed int) {
	if connectedUsers, err := Settings.store.GetConnectedUsersCount(); err != nil || connectedUsers >= int64(connectedUsersAllowed) {
		return
	}

	token, _ := Settings.store.GetTokenForMattermostUser(userID)
	if token == nil {
		FakeConnectUserForLoadTest(userID)
		return
	}
}

func FakeConnectUsersIfNeeded(userIDs []string, connectedUsersAllowed int) {
	connectedUsers, err := Settings.store.GetConnectedUsersCount()
	if err != nil {
		return
	}

	allowed := int64(connectedUsersAllowed)
	for i, userID := range userIDs {
		if (connectedUsers + int64(i)) < allowed {
			token, _ := Settings.store.GetTokenForMattermostUser(userID)
			if token == nil {
				FakeConnectUserForLoadTest(userID)
			}
		}
	}
}

func FakeConnectSysadminAndBot(connectedUsersAllowed int) {
	mmUsersIDs := []string{}
	mmAdmin, appErr := Settings.api.GetUserByUsername("sysadmin")
	if appErr != nil {
		Settings.api.LogWarn("Unable to get MM sysadmin during setup load test", "error", appErr.Error())
		return
	}
	mmUsersIDs = append(mmUsersIDs, mmAdmin.Id)

	msBot, appErr := Settings.api.GetUserByUsername("msteams")
	if appErr != nil {
		Settings.api.LogWarn("Unable to get MS Teams bot user during setup load test", "error", appErr.Error())
		return
	}

	mmUsersIDs = append(mmUsersIDs, msBot.Id)
	FakeConnectUsersIfNeeded(mmUsersIDs, connectedUsersAllowed)
}
