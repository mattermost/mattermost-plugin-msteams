package store

import (
	"encoding/json"
	"errors"

	"github.com/mattermost/mattermost-server/v6/plugin"
	"golang.org/x/oauth2"
)

const (
	avatarCacheTime = 300
)

type ChannelLink struct {
	MattermostTeam    string
	MattermostChannel string
	MSTeamsTeam       string
	MSTeamsChannel    string
}

type Store interface {
	GetAvatarCache(userID string) ([]byte, error)
	SetAvatarCache(userID string, photo []byte) error
	GetLinkByChannelID(channelID string) (*ChannelLink, error)
	GetLinkByMSTeamsChannelID(teamID, channelID string) (*ChannelLink, error)
	DeleteLinkByChannelID(channelID string) error
	StoreChannelLink(link *ChannelLink) error
	TeamsToMattermostPostId(chatID string, postID string) (string, error)
	MattermostToTeamsPostId(postID string) (string, error)
	LinkPosts(mattermostPostID, chatOrChannelID, teamsPostID string) error
	GetTokenForMattermostUser(userID string) (*oauth2.Token, error)
	SetTokenForMattermostUser(userID string, token *oauth2.Token) error
	SetTeamsToMattermostUserId(teamsUserID, mattermostUserId string) error
	SetMattermostToTeamsUserId(mattermostUserId, teamsUserId string) error
	TeamsToMattermostUserId(userID string) (string, error)
	MattermostToTeamsUserId(userID string) (string, error)
	CheckEnabledTeamByTeamId(teamId string) bool
}

type StoreImpl struct {
	api          plugin.API
	enabledTeams func() []string
}

func New(api plugin.API, enabledTeams func() []string) *StoreImpl {
	return &StoreImpl{
		api:          api,
		enabledTeams: enabledTeams,
	}
}

func (s *StoreImpl) GetAvatarCache(userID string) ([]byte, error) {
	data, appErr := s.api.KVGet(avatarKey(userID))
	if appErr != nil {
		return nil, appErr
	}
	return data, nil
}

func (s *StoreImpl) SetAvatarCache(userID string, photo []byte) error {
	appErr := s.api.KVSetWithExpiry(avatarKey(userID), photo, avatarCacheTime)
	if appErr != nil {
		return appErr
	}
	return nil

}

func (s *StoreImpl) GetLinkByChannelID(channelID string) (*ChannelLink, error) {
	linkdata, appErr := s.api.KVGet(channelsLinkedKey(channelID))
	if appErr != nil {
		return nil, appErr
	}
	var link ChannelLink
	err := json.Unmarshal(linkdata, &link)
	if err != nil {
		return nil, err
	}
	if !s.CheckEnabledTeamByTeamId(link.MattermostTeam) {
		return nil, errors.New("link not enabled for this team")
	}
	return &link, nil
}

func (s *StoreImpl) GetLinkByMSTeamsChannelID(teamID, channelID string) (*ChannelLink, error) {
	linkdata, appErr := s.api.KVGet(channelsLinkedByMSTeamsKey(teamID, channelID))
	if appErr != nil {
		return nil, appErr
	}
	var link ChannelLink
	err := json.Unmarshal(linkdata, &link)
	if err != nil {
		return nil, err
	}
	if !s.CheckEnabledTeamByTeamId(link.MattermostTeam) {
		return nil, errors.New("link not enabled for this team")
	}
	return &link, nil
}

func (s *StoreImpl) DeleteLinkByChannelID(channelID string) error {
	_, err := s.GetLinkByChannelID(channelID)
	if err != nil {
		return err
	}

	appErr := s.api.KVDelete(channelsLinkedKey(channelID))
	if appErr != nil {
		return appErr
	}

	return nil
}

func (s *StoreImpl) StoreChannelLink(link *ChannelLink) error {
	linkdata, err := json.Marshal(link)
	if err != nil {
		return err
	}

	appErr := s.api.KVSet(channelsLinkedKey(link.MattermostChannel), linkdata)
	if appErr != nil {
		return appErr
	}
	appErr = s.api.KVSet(channelsLinkedByMSTeamsKey(link.MSTeamsTeam, link.MSTeamsChannel), linkdata)
	if appErr != nil {
		_ = s.api.KVDelete(channelsLinkedKey(link.MattermostChannel))
		return appErr
	}
	return nil
}

func (s *StoreImpl) TeamsToMattermostUserId(userID string) (string, error) {
	data, err := s.api.KVGet(teamsMattermostUserKey(userID))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *StoreImpl) MattermostToTeamsUserId(userID string) (string, error) {
	data, err := s.api.KVGet(mattermostTeamsUserKey(userID))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *StoreImpl) TeamsToMattermostPostId(chatID string, postID string) (string, error) {
	data, err := s.api.KVGet(teamsMattermostPostKey(chatID, postID))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *StoreImpl) MattermostToTeamsPostId(postID string) (string, error) {
	data, err := s.api.KVGet(mattermostTeamsPostKey(postID))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *StoreImpl) LinkPosts(mattermostPostID, teamsChatOrChannelID, teamsPostID string) error {
	err := s.api.KVSet(mattermostTeamsPostKey(mattermostPostID), []byte(teamsPostID))
	if err != nil {
		return err
	}
	err = s.api.KVSet(teamsMattermostPostKey(teamsChatOrChannelID, teamsPostID), []byte(mattermostPostID))
	if err != nil {
		_ = s.api.KVDelete(mattermostTeamsPostKey(mattermostPostID))
		return err
	}
	return nil
}

func (s *StoreImpl) GetTokenForMattermostUser(userID string) (*oauth2.Token, error) {
	tokendata, appErr := s.api.KVGet(tokenForMattermostUserKey(userID))
	if appErr != nil {
		return nil, appErr
	}
	var token oauth2.Token
	err := json.Unmarshal(tokendata, &token)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *StoreImpl) SetTokenForMattermostUser(userID string, token *oauth2.Token) error {
	tokendata, err := json.Marshal(token)
	if err != nil {
		return err
	}
	appErr := s.api.KVSet(tokenForMattermostUserKey(userID), tokendata)
	if appErr != nil {
		return appErr
	}
	return nil
}

func (s *StoreImpl) SetTeamsToMattermostUserId(teamsUserID, mattermostUserId string) error {
	appErr := s.api.KVSet(teamsMattermostUserKey(teamsUserID), []byte(mattermostUserId))
	if appErr != nil {
		return appErr
	}
	return nil
}

func (s *StoreImpl) SetMattermostToTeamsUserId(mattermostUserId, teamsUserID string) error {
	appErr := s.api.KVSet(mattermostTeamsUserKey(mattermostUserId), []byte(teamsUserID))
	if appErr != nil {
		return appErr
	}
	return nil
}

func (s *StoreImpl) CheckEnabledTeamByTeamId(teamId string) bool {
	if len(s.enabledTeams()) == 1 && s.enabledTeams()[0] == "" {
		return true
	}
	team, appErr := s.api.GetTeam(teamId)
	if appErr != nil {
		return false
	}
	isTeamEnabled := false
	for _, enabledTeam := range s.enabledTeams() {
		if team.Name == enabledTeam {
			isTeamEnabled = true
			break
		}
	}
	return isTeamEnabled
}
