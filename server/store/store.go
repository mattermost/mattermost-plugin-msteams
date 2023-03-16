//go:generate mockery --name=Store
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"golang.org/x/oauth2"
)

const (
	avatarCacheTime = 300
	avatarKey       = "avatar_"
)

type ChannelLink struct {
	MattermostTeam    string
	MattermostChannel string
	MSTeamsTeam       string
	MSTeamsChannel    string
}

type PostInfo struct {
	MattermostID        string
	MSTeamsID           string
	MSTeamsChannel      string
	MSTeamsLastUpdateAt time.Time
}

type Store interface {
	Init() error
	GetAvatarCache(userID string) ([]byte, error)
	SetAvatarCache(userID string, photo []byte) error
	GetLinkByChannelID(channelID string) (*ChannelLink, error)
	GetLinkByMSTeamsChannelID(teamID, channelID string) (*ChannelLink, error)
	DeleteLinkByChannelID(channelID string) error
	StoreChannelLink(link *ChannelLink) error
	GetPostInfoByMSTeamsID(chatID string, postID string) (*PostInfo, error)
	GetPostInfoByMattermostID(postID string) (*PostInfo, error)
	LinkPosts(postInfo PostInfo) error
	GetTokenForMattermostUser(userID string) (*oauth2.Token, error)
	GetTokenForMSTeamsUser(userID string) (*oauth2.Token, error)
	SetUserInfo(userID string, msTeamsUserID string, token *oauth2.Token) error
	TeamsToMattermostUserID(userID string) (string, error)
	MattermostToTeamsUserID(userID string) (string, error)
	CheckEnabledTeamByTeamID(teamID string) bool
}

type SQLStore struct {
	api           plugin.API
	enabledTeams  func() []string
	encryptionKey func() []byte
	db            *sql.DB
	driverName    string
}

func New(db *sql.DB, driverName string, api plugin.API, enabledTeams func() []string, encryptionKey func() []byte) *SQLStore {
	return &SQLStore{
		db:            db,
		driverName:    driverName,
		api:           api,
		enabledTeams:  enabledTeams,
		encryptionKey: encryptionKey,
	}
}

func (s *SQLStore) Init() error {
	_, err := s.db.Exec("CREATE TABLE IF NOT EXISTS msteamssync_links (mmChannelID VARCHAR PRIMARY KEY, mmTeamID VARCHAR, msTeamsChannelID VARCHAR, msTeamsTeamID VARCHAR)")
	if err != nil {
		return err
	}

	_, err = s.db.Exec("CREATE INDEX IF NOT EXISTS idx_msteamssync_links_msteamsteamid_msteamschannelid ON msteamssync_links (msTeamsTeamID, msTeamsChannelID)")
	if err != nil {
		return err
	}

	_, err = s.db.Exec("CREATE TABLE IF NOT EXISTS msteamssync_users (mmUserID VARCHAR PRIMARY KEY, msTeamsUserID VARCHAR, token TEXT)")
	if err != nil {
		return err
	}

	_, err = s.db.Exec("CREATE INDEX IF NOT EXISTS idx_msteamssync_users_msteamsuserid ON msteamssync_users (msTeamsUserID)")
	if err != nil {
		return err
	}

	_, err = s.db.Exec("CREATE TABLE IF NOT EXISTS msteamssync_posts (mmPostID VARCHAR PRIMARY KEY, msTeamsPostID VARCHAR, msTeamsChannelID VARCHAR, msTeamsLastUpdateAt BIGINT)")
	if err != nil {
		return err
	}

	_, err = s.db.Exec("CREATE INDEX IF NOT EXISTS idx_msteamssync_posts_msteamschannelid_msteamspostid ON msteamssync_posts (msTeamsChannelID, msTeamsPostID)")
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) GetAvatarCache(userID string) ([]byte, error) {
	data, appErr := s.api.KVGet(avatarKey + userID)
	if appErr != nil {
		return nil, appErr
	}
	return data, nil
}

func (s *SQLStore) SetAvatarCache(userID string, photo []byte) error {
	appErr := s.api.KVSetWithExpiry(avatarKey+userID, photo, avatarCacheTime)
	if appErr != nil {
		return appErr
	}
	return nil
}

func (s *SQLStore) GetLinkByChannelID(channelID string) (*ChannelLink, error) {
	query := s.getQueryBuilder().Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID").From("msteamssync_links").Where(sq.Eq{"mmChannelID": channelID})
	row := query.QueryRow()
	var link ChannelLink
	err := row.Scan(&link.MattermostChannel, &link.MattermostTeam, &link.MSTeamsChannel, &link.MSTeamsTeam)
	if err != nil {
		return nil, err
	}

	if !s.CheckEnabledTeamByTeamID(link.MattermostTeam) {
		return nil, errors.New("link not enabled for this team")
	}
	return &link, nil
}

func (s *SQLStore) GetLinkByMSTeamsChannelID(teamID, channelID string) (*ChannelLink, error) {
	query := s.getQueryBuilder().Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID").From("msteamssync_links").Where(sq.Eq{"msTeamsChannelID": channelID})
	row := query.QueryRow()
	var link ChannelLink
	err := row.Scan(&link.MattermostChannel, &link.MattermostTeam, &link.MSTeamsChannel, &link.MSTeamsTeam)
	if err != nil {
		return nil, err
	}
	if !s.CheckEnabledTeamByTeamID(link.MattermostTeam) {
		return nil, errors.New("link not enabled for this team")
	}
	return &link, nil
}

func (s *SQLStore) DeleteLinkByChannelID(channelID string) error {
	query := s.getQueryBuilder().Delete("msteamssync_links").Where(sq.Eq{"mmChannelID": channelID})
	_, err := query.Exec()
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) StoreChannelLink(link *ChannelLink) error {
	query := s.getQueryBuilder().Insert("msteamssync_links").Columns("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID").Values(link.MattermostChannel, link.MattermostTeam, link.MSTeamsChannel, link.MSTeamsTeam)
	_, err := query.Exec()
	if err != nil {
		return err
	}
	if !s.CheckEnabledTeamByTeamID(link.MattermostTeam) {
		return errors.New("link not enabled for this team")
	}
	return nil
}

func (s *SQLStore) TeamsToMattermostUserID(userID string) (string, error) {
	query := s.getQueryBuilder().Select("mmUserID").From("msteamssync_users").Where(sq.Eq{"msTeamsUserID": userID})
	row := query.QueryRow()
	var mmUserID string
	err := row.Scan(&mmUserID)
	if err != nil {
		return "", err
	}
	return mmUserID, nil
}

func (s *SQLStore) MattermostToTeamsUserID(userID string) (string, error) {
	query := s.getQueryBuilder().Select("msTeamsUserID").From("msteamssync_users").Where(sq.Eq{"mmUserID": userID})
	row := query.QueryRow()
	var msTeamsUserID string
	err := row.Scan(&msTeamsUserID)
	if err != nil {
		return "", err
	}
	return msTeamsUserID, nil
}

func (s *SQLStore) GetPostInfoByMSTeamsID(chatID string, postID string) (*PostInfo, error) {
	query := s.getQueryBuilder().Select("mmPostID, msTeamsLastUpdateAt").From("msteamssync_posts").Where(sq.Eq{"msTeamsPostID": postID, "msTeamsChannelID": chatID})
	row := query.QueryRow()
	var lastUpdateAt int64
	postInfo := PostInfo{
		MSTeamsID:      postID,
		MSTeamsChannel: chatID,
	}
	err := row.Scan(&postInfo.MattermostID, &lastUpdateAt)
	if err != nil {
		return nil, err
	}
	postInfo.MSTeamsLastUpdateAt = time.UnixMicro(lastUpdateAt)
	return &postInfo, nil
}

func (s *SQLStore) GetPostInfoByMattermostID(postID string) (*PostInfo, error) {
	query := s.getQueryBuilder().Select("msTeamsPostID, msTeamsChannelID, msTeamsLastUpdateAt").From("msteamssync_posts").Where(sq.Eq{"mmPostID": postID})
	row := query.QueryRow()
	var lastUpdateAt int64
	postInfo := PostInfo{
		MattermostID: postID,
	}
	err := row.Scan(&postInfo.MSTeamsID, &postInfo.MSTeamsChannel, lastUpdateAt)
	if err != nil {
		return nil, err
	}
	postInfo.MSTeamsLastUpdateAt = time.UnixMicro(lastUpdateAt)
	return &postInfo, nil
}

func (s *SQLStore) LinkPosts(postInfo PostInfo) error {
	if s.driverName == "postgres" {
		_, err := s.getQueryBuilder().Insert("msteamssync_posts").Columns("mmPostID, msTeamsPostID, msTeamsChannelID, msTeamsLastUpdateAt").Values(
			postInfo.MattermostID,
			postInfo.MSTeamsID,
			postInfo.MSTeamsChannel,
			postInfo.MSTeamsLastUpdateAt.UnixMicro(),
		).Suffix("ON CONFLICT (mmPostID) DO UPDATE SET msTeamsPostID = EXCLUDED.msTeamsPostID, msTeamsChannelID = EXCLUDED.msTeamsChannelID, msTeamsLastUpdateAt = EXCLUDED.msTeamsLastUpdateAt").Exec()
		if err != nil {
			return err
		}
	} else {
		_, err := s.getQueryBuilder().Replace("msteamssync_posts").Columns("mmPostID, msTeamsPostID, msTeamsChannelID, msTeamsLastUpdateAt").Values(
			postInfo.MattermostID,
			postInfo.MSTeamsID,
			postInfo.MSTeamsChannel,
			postInfo.MSTeamsLastUpdateAt.UnixMicro(),
		).Exec()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLStore) GetTokenForMattermostUser(userID string) (*oauth2.Token, error) {
	query := s.getQueryBuilder().Select("token").From("msteamssync_users").Where(sq.Eq{"mmUserID": userID}, sq.NotEq{"token": ""})
	row := query.QueryRow()
	var encryptedToken string
	err := row.Scan(&encryptedToken)
	if err != nil {
		return nil, err
	}

	if encryptedToken == "" {
		return nil, errors.New("token not found")
	}

	tokendata, err := decrypt(s.encryptionKey(), encryptedToken)
	if err != nil {
		return nil, err
	}

	var token oauth2.Token
	err = json.Unmarshal([]byte(tokendata), &token)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *SQLStore) GetTokenForMSTeamsUser(userID string) (*oauth2.Token, error) {
	query := s.getQueryBuilder().Select("token").From("msteamssync_users").Where(sq.Eq{"msTeamsUserID": userID}, sq.NotEq{"token": ""})
	row := query.QueryRow()
	var encryptedToken string
	err := row.Scan(&encryptedToken)
	if err != nil {
		return nil, err
	}

	if encryptedToken == "" {
		return nil, errors.New("token not found")
	}

	tokendata, err := decrypt(s.encryptionKey(), encryptedToken)
	if err != nil {
		return nil, err
	}

	var token oauth2.Token
	err = json.Unmarshal([]byte(tokendata), &token)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *SQLStore) SetUserInfo(userID string, msTeamsUserID string, token *oauth2.Token) error {
	tokendata := []byte{}
	if token != nil {
		var err error
		tokendata, err = json.Marshal(token)
		if err != nil {
			return err
		}
	}

	encryptedToken, err := encrypt(s.encryptionKey(), string(tokendata))
	if err != nil {
		return err
	}

	if s.driverName == "postgres" {
		_, err := s.getQueryBuilder().Insert("msteamssync_users").Columns("mmUserID, msTeamsUserID, token").Values(userID, msTeamsUserID, encryptedToken).Suffix("ON CONFLICT (mmUserID) DO UPDATE SET msTeamsUserID = EXCLUDED.msTeamsUserID, token = EXCLUDED.token").Exec()
		if err != nil {
			return err
		}
	} else {
		_, err := s.getQueryBuilder().Replace("msteamssync_users").Columns("mmUserID, msTeamsUserID, token").Values(userID, msTeamsUserID, encryptedToken).Exec()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLStore) CheckEnabledTeamByTeamID(teamID string) bool {
	if len(s.enabledTeams()) == 1 && s.enabledTeams()[0] == "" {
		return true
	}
	team, appErr := s.api.GetTeam(teamID)
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

func (s *SQLStore) getQueryBuilder() sq.StatementBuilderType {
	builder := sq.StatementBuilder
	if s.driverName == "postgres" {
		builder = builder.PlaceholderFormat(sq.Dollar)
	}

	return builder.RunWith(s.db)
}
