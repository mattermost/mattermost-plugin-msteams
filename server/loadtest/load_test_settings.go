package loadtest

import (
	"github.com/mattermost/mattermost-plugin-msteams/server/store"
	plugin "github.com/mattermost/mattermost/server/public/plugin"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
)

type LoadTestUserTokenData struct {
	AccessToken string
	UserId      string
	TeamsUserId string
}

type LoadTestUserTokenMap map[string]LoadTestUserTokenData

type LoadTestSettings struct {
	api                   plugin.API
	store                 store.Store
	logService            *pluginapi.LogService
	applicationId         string
	secret                string
	webhookSecret         string
	baseUrl               string
	tenantId              string
	Enabled               bool
	userTokenMap          LoadTestUserTokenMap
	maxIncomingPosts      int
	minIncomingPosts      int
	simulateIncomingPosts bool
}

func (s *LoadTestSettings) AddTokenToMap(accessToken, mmUserId, msUserId string) {
	s.userTokenMap[accessToken] = LoadTestUserTokenData{
		AccessToken: accessToken,
		UserId:      mmUserId,
		TeamsUserId: msUserId,
	}
}

func (s *LoadTestSettings) MapHasToken(accessToken string) bool {
	_, ok := s.userTokenMap[accessToken]
	return ok
}

func (s *LoadTestSettings) GetUserTokenData(accessToken string) (LoadTestUserTokenData, bool) {
	data, ok := s.userTokenMap[accessToken]
	return data, ok
}
