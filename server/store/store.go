//go:generate mockery --name=Store
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"golang.org/x/oauth2"
)

const (
	avatarCacheTime              = 300
	avatarKey                    = "avatar_"
	connectionPromptKey          = "connect_"
	subscriptionRefreshTimeLimit = 5 * time.Minute
	subscriptionTypeUser         = "user"
	subscriptionTypeChannel      = "channel"
	subscriptionTypeAllChats     = "allChats"
)

type Store interface {
	Init() error
	GetAvatarCache(userID string) ([]byte, error)
	SetAvatarCache(userID string, photo []byte) error
	GetLinkByChannelID(channelID string) (*storemodels.ChannelLink, error)
	ListChannelLinks() ([]storemodels.ChannelLink, error)
	GetLinkByMSTeamsChannelID(teamID, channelID string) (*storemodels.ChannelLink, error)
	DeleteLinkByChannelID(channelID string) error
	StoreChannelLink(link *storemodels.ChannelLink) error
	GetPostInfoByMSTeamsID(chatID string, postID string) (*storemodels.PostInfo, error)
	GetPostInfoByMattermostID(postID string) (*storemodels.PostInfo, error)
	LinkPosts(postInfo storemodels.PostInfo) error
	GetTokenForMattermostUser(userID string) (*oauth2.Token, error)
	GetTokenForMSTeamsUser(userID string) (*oauth2.Token, error)
	SetUserInfo(userID string, msTeamsUserID string, token *oauth2.Token) error
	DeleteUserInfo(mmUserID string) error
	TeamsToMattermostUserID(userID string) (string, error)
	MattermostToTeamsUserID(userID string) (string, error)
	CheckEnabledTeamByTeamID(teamID string) bool
	ListGlobalSubscriptionsToCheck() ([]storemodels.GlobalSubscription, error)
	ListChatSubscriptionsToCheck() ([]storemodels.ChatSubscription, error)
	ListChannelSubscriptionsToCheck() ([]storemodels.ChannelSubscription, error)
	SaveGlobalSubscription(storemodels.GlobalSubscription) error
	SaveChatSubscription(storemodels.ChatSubscription) error
	SaveChannelSubscription(storemodels.ChannelSubscription) error
	UpdateSubscriptionExpiresOn(subscriptionID string, expiresOn time.Time) error
	DeleteSubscription(subscriptionID string) error
	GetChannelSubscription(subscriptionID string) (*storemodels.ChannelSubscription, error)
	GetChatSubscription(subscriptionID string) (*storemodels.ChatSubscription, error)
	GetGlobalSubscription(subscriptionID string) (*storemodels.GlobalSubscription, error)
	GetSubscriptionType(subscriptionID string) (string, error)
	StoreDMAndGMChannelPromptTime(channelID, userID string, timestamp time.Time) error
	GetDMAndGMChannelPromptTime(channelID, userID string) (time.Time, error)
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

func (s *SQLStore) createIndexForMySQL(tableName, indexName, columnList string) error {
	// TODO: Try to do this using only one query
	query := `SELECT EXISTS(
			SELECT DISTINCT index_name FROM information_schema.statistics 
			WHERE table_schema = DATABASE()
			AND table_name = 'tableName' AND index_name = 'indexName'
		)`

	query = strings.ReplaceAll(query, "tableName", tableName)
	query = strings.ReplaceAll(query, "indexName", indexName)
	rows, err := s.db.Query(query)
	if err != nil {
		return err
	}

	var result int
	if rows.Next() {
		if scanErr := rows.Scan(&result); scanErr != nil {
			return scanErr
		}
	}

	if result == 0 {
		indexQuery := "CREATE INDEX indexName on tableName(columnList)"
		indexQuery = strings.ReplaceAll(indexQuery, "tableName", tableName)
		indexQuery = strings.ReplaceAll(indexQuery, "indexName", indexName)
		indexQuery = strings.ReplaceAll(indexQuery, "columnList", columnList)
		_, err = s.db.Exec(indexQuery)
	}

	return err
}

func (s *SQLStore) addColumnForMySQL(tableName, columnName, columnDefinition string) error {
	// TODO: Try to do this using only one query
	query := `SELECT EXISTS(
			SELECT NULL FROM INFORMATION_SCHEMA.COLUMNS WHERE table_name = 'tableName'
			AND table_schema = DATABASE()
			AND column_name = 'columnName'
		)`

	query = strings.ReplaceAll(query, "tableName", tableName)
	query = strings.ReplaceAll(query, "columnName", columnName)
	rows, err := s.db.Query(query)
	if err != nil {
		return err
	}

	var result int
	if rows.Next() {
		if scanErr := rows.Scan(&result); scanErr != nil {
			return scanErr
		}
	}

	if result == 0 {
		alterQuery := fmt.Sprintf("ALTER TABLE %s ADD %s %s", tableName, columnName, columnDefinition)
		_, err = s.db.Exec(alterQuery)
	}

	return err
}

func (s *SQLStore) createTable(tableName, columnList string) error {
	if _, err := s.db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, columnList)); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) createIndex(tableName, indexName, columnList string) error {
	var err error
	if s.driverName == model.DatabaseDriverPostgres {
		_, err = s.db.Exec(fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", indexName, tableName, columnList))
	} else {
		err = s.createIndexForMySQL(tableName, indexName, columnList)
	}

	return err
}

func (s *SQLStore) addColumn(tableName, columnName, columnDefinition string) error {
	if s.driverName == model.DatabaseDriverPostgres {
		if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s", tableName, columnName, columnDefinition)); err != nil {
			return err
		}
	} else if err := s.addColumnForMySQL(tableName, columnName, columnDefinition); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) addPrimaryKey(tableName, columnList string) error {
	if s.driverName == model.DatabaseDriverPostgres {
		rows, err := s.db.Query(fmt.Sprintf("SELECT constraint_name from information_schema.table_constraints where table_name = '%s' and constraint_type='PRIMARY KEY'", tableName))
		if err != nil {
			return err
		}

		var constraintName string
		if rows.Next() {
			if scanErr := rows.Scan(&constraintName); scanErr != nil {
				return scanErr
			}
		}

		if constraintName == "" {
			if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD PRIMARY KEY(%s)", tableName, columnList)); err != nil {
				return err
			}
		} else if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s, ADD PRIMARY KEY(%s)", tableName, constraintName, columnList)); err != nil {
			return err
		}
	} else {
		if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s DROP PRIMARY KEY", tableName)); err != nil {
			s.api.LogDebug("Error in dropping primary key", "Error", err.Error())
		}

		if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD PRIMARY KEY(%s)", tableName, columnList)); err != nil {
			return err
		}
	}

	return nil
}

func (s *SQLStore) Init() error {
	if err := s.createTable("msteamssync_subscriptions", "subscriptionID VARCHAR(255) PRIMARY KEY, type VARCHAR(255), msTeamsTeamID VARCHAR(255), msTeamsChannelID VARCHAR(255), msTeamsUserID VARCHAR(255), secret VARCHAR(255), expiresOn BIGINT"); err != nil {
		return err
	}

	if err := s.createTable("msteamssync_links", "mmChannelID VARCHAR(255) PRIMARY KEY, mmTeamID VARCHAR(255), msTeamsChannelID VARCHAR(255), msTeamsTeamID VARCHAR(255), creator VARCHAR(255)"); err != nil {
		return err
	}

	if err := s.addColumn("msteamssync_links", "creator", "VARCHAR(255)"); err != nil {
		return err
	}

	if err := s.createTable("msteamssync_users", "mmUserID VARCHAR(255) PRIMARY KEY, msTeamsUserID VARCHAR(255), token TEXT"); err != nil {
		return err
	}

	if err := s.addPrimaryKey("msteamssync_users", "mmUserID, msTeamsUserID"); err != nil {
		return err
	}

	if err := s.createTable("msteamssync_posts", "mmPostID VARCHAR(255) PRIMARY KEY, msTeamsPostID VARCHAR(255), msTeamsChannelID VARCHAR(255), msTeamsLastUpdateAt BIGINT"); err != nil {
		return err
	}

	if err := s.createIndex("msteamssync_links", "idx_msteamssync_links_msteamsteamid_msteamschannelid", "msTeamsTeamID, msTeamsChannelID"); err != nil {
		return err
	}

	if err := s.createIndex("msteamssync_users", "idx_msteamssync_users_msteamsuserid", "msTeamsUserID"); err != nil {
		return err
	}

	return s.createIndex("msteamssync_posts", "idx_msteamssync_posts_msteamschannelid_msteamspostid", "msTeamsChannelID, msTeamsPostID")
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

func (s *SQLStore) GetLinkByChannelID(channelID string) (*storemodels.ChannelLink, error) {
	query := s.getQueryBuilder().Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator").From("msteamssync_links").Where(sq.Eq{"mmChannelID": channelID})
	row := query.QueryRow()
	var link storemodels.ChannelLink
	err := row.Scan(&link.MattermostChannel, &link.MattermostTeam, &link.MSTeamsChannel, &link.MSTeamsTeam, &link.Creator)
	if err != nil {
		return nil, err
	}

	if !s.CheckEnabledTeamByTeamID(link.MattermostTeam) {
		return nil, errors.New("link not enabled for this team")
	}
	return &link, nil
}

func (s *SQLStore) ListChannelLinks() ([]storemodels.ChannelLink, error) {
	rows, err := s.getQueryBuilder().Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator").From("msteamssync_links").Query()
	if err != nil {
		return nil, err
	}

	links := []storemodels.ChannelLink{}
	for rows.Next() {
		var link storemodels.ChannelLink
		err := rows.Scan(&link.MattermostChannel, &link.MattermostTeam, &link.MSTeamsChannel, &link.MSTeamsTeam, &link.Creator)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}

	return links, nil
}

func (s *SQLStore) GetLinkByMSTeamsChannelID(teamID, channelID string) (*storemodels.ChannelLink, error) {
	query := s.getQueryBuilder().Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator").From("msteamssync_links").Where(sq.Eq{"msTeamsTeamID": teamID, "msTeamsChannelID": channelID})
	row := query.QueryRow()
	var link storemodels.ChannelLink
	err := row.Scan(&link.MattermostChannel, &link.MattermostTeam, &link.MSTeamsChannel, &link.MSTeamsTeam, &link.Creator)
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

func (s *SQLStore) StoreChannelLink(link *storemodels.ChannelLink) error {
	query := s.getQueryBuilder().Insert("msteamssync_links").Columns("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator").Values(link.MattermostChannel, link.MattermostTeam, link.MSTeamsChannel, link.MSTeamsTeam, link.Creator)
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

func (s *SQLStore) GetPostInfoByMSTeamsID(chatID string, postID string) (*storemodels.PostInfo, error) {
	query := s.getQueryBuilder().Select("mmPostID, msTeamsLastUpdateAt").From("msteamssync_posts").Where(sq.Eq{"msTeamsPostID": postID, "msTeamsChannelID": chatID})
	row := query.QueryRow()
	var lastUpdateAt int64
	postInfo := storemodels.PostInfo{
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

func (s *SQLStore) GetPostInfoByMattermostID(postID string) (*storemodels.PostInfo, error) {
	query := s.getQueryBuilder().Select("msTeamsPostID, msTeamsChannelID, msTeamsLastUpdateAt").From("msteamssync_posts").Where(sq.Eq{"mmPostID": postID})
	row := query.QueryRow()
	var lastUpdateAt int64
	postInfo := storemodels.PostInfo{
		MattermostID: postID,
	}
	err := row.Scan(&postInfo.MSTeamsID, &postInfo.MSTeamsChannel, &lastUpdateAt)
	if err != nil {
		return nil, err
	}
	postInfo.MSTeamsLastUpdateAt = time.UnixMicro(lastUpdateAt)
	return &postInfo, nil
}

func (s *SQLStore) LinkPosts(postInfo storemodels.PostInfo) error {
	if s.driverName == "postgres" {
		if _, err := s.getQueryBuilder().Insert("msteamssync_posts").Columns("mmPostID, msTeamsPostID, msTeamsChannelID, msTeamsLastUpdateAt").Values(
			postInfo.MattermostID,
			postInfo.MSTeamsID,
			postInfo.MSTeamsChannel,
			postInfo.MSTeamsLastUpdateAt.UnixMicro(),
		).Suffix("ON CONFLICT (mmPostID) DO UPDATE SET msTeamsPostID = EXCLUDED.msTeamsPostID, msTeamsChannelID = EXCLUDED.msTeamsChannelID, msTeamsLastUpdateAt = EXCLUDED.msTeamsLastUpdateAt").Exec(); err != nil {
			return err
		}
	} else {
		if _, err := s.getQueryBuilder().Replace("msteamssync_posts").Columns("mmPostID, msTeamsPostID, msTeamsChannelID, msTeamsLastUpdateAt").Values(
			postInfo.MattermostID,
			postInfo.MSTeamsID,
			postInfo.MSTeamsChannel,
			postInfo.MSTeamsLastUpdateAt.UnixMicro(),
		).Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLStore) GetTokenForMattermostUser(userID string) (*oauth2.Token, error) {
	query := s.getQueryBuilder().Select("token").From("msteamssync_users").Where(sq.Eq{"mmUserID": userID}).Where(sq.NotEq{"token": ""})
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

	if tokendata == "" {
		return nil, errors.New("token not found")
	}

	var token oauth2.Token
	err = json.Unmarshal([]byte(tokendata), &token)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *SQLStore) GetTokenForMSTeamsUser(userID string) (*oauth2.Token, error) {
	query := s.getQueryBuilder().Select("token").From("msteamssync_users").Where(sq.Eq{"msTeamsUserID": userID}).Where(sq.NotEq{"token": ""})
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

	if tokendata == "" {
		return nil, errors.New("token not found")
	}

	var token oauth2.Token
	err = json.Unmarshal([]byte(tokendata), &token)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *SQLStore) SetUserInfo(userID string, msTeamsUserID string, token *oauth2.Token) error {
	var encryptedToken string
	if token != nil {
		var err error
		var tokendata []byte
		tokendata, err = json.Marshal(token)
		if err != nil {
			return err
		}

		encryptedToken, err = encrypt(s.encryptionKey(), string(tokendata))
		if err != nil {
			return err
		}
	}

	if s.driverName == "postgres" {
		if _, err := s.getQueryBuilder().Insert("msteamssync_users").Columns("mmUserID, msTeamsUserID, token").Values(userID, msTeamsUserID, encryptedToken).Suffix("ON CONFLICT (mmUserID, msTeamsUserID) DO UPDATE SET token = EXCLUDED.token").Exec(); err != nil {
			return err
		}
	} else {
		if _, err := s.getQueryBuilder().Replace("msteamssync_users").Columns("mmUserID, msTeamsUserID, token").Values(userID, msTeamsUserID, encryptedToken).Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLStore) DeleteUserInfo(mmUserID string) error {
	if _, err := s.getQueryBuilder().Delete("msteamssync_users").Where(sq.Eq{"mmUserID": mmUserID}).Exec(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) ListChatSubscriptionsToCheck() ([]storemodels.ChatSubscription, error) {
	expireTime := time.Now().Add(subscriptionRefreshTimeLimit).UnixMicro()
	query := s.getQueryBuilder().Select("subscriptionID, msTeamsUserID, secret, expiresOn").From("msteamssync_subscriptions").Where(sq.Eq{"type": subscriptionTypeUser}).Where(sq.Lt{"expiresOn": expireTime})
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}

	result := []storemodels.ChatSubscription{}
	for rows.Next() {
		var subscription storemodels.ChatSubscription
		var expiresOn int64
		if scanErr := rows.Scan(&subscription.SubscriptionID, &subscription.UserID, &subscription.Secret, &expiresOn); scanErr != nil {
			return nil, scanErr
		}
		subscription.ExpiresOn = time.UnixMicro(expiresOn)
		result = append(result, subscription)
	}
	return result, nil
}

func (s *SQLStore) ListChannelSubscriptionsToCheck() ([]storemodels.ChannelSubscription, error) {
	expireTime := time.Now().Add(subscriptionRefreshTimeLimit).UnixMicro()
	query := s.getQueryBuilder().Select("subscriptionID, msTeamsChannelID, msTeamsTeamID, secret, expiresOn").From("msteamssync_subscriptions").Where(sq.Eq{"type": subscriptionTypeChannel}).Where(sq.Lt{"expiresOn": expireTime})
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}

	result := []storemodels.ChannelSubscription{}
	for rows.Next() {
		var subscription storemodels.ChannelSubscription
		var expiresOn int64
		if scanErr := rows.Scan(&subscription.SubscriptionID, &subscription.ChannelID, &subscription.TeamID, &subscription.Secret, &expiresOn); scanErr != nil {
			return nil, scanErr
		}
		subscription.ExpiresOn = time.UnixMicro(expiresOn)
		result = append(result, subscription)
	}
	return result, nil
}

func (s *SQLStore) ListGlobalSubscriptionsToCheck() ([]storemodels.GlobalSubscription, error) {
	expireTime := time.Now().Add(subscriptionRefreshTimeLimit).UnixMicro()
	query := s.getQueryBuilder().Select("subscriptionID, type, secret, expiresOn").From("msteamssync_subscriptions").Where(sq.Eq{"type": subscriptionTypeAllChats}).Where(sq.Lt{"expiresOn": expireTime})
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}

	result := []storemodels.GlobalSubscription{}
	for rows.Next() {
		var subscription storemodels.GlobalSubscription
		var expiresOn int64
		if scanErr := rows.Scan(&subscription.SubscriptionID, &subscription.Type, &subscription.Secret, &expiresOn); scanErr != nil {
			return nil, scanErr
		}
		subscription.ExpiresOn = time.UnixMicro(expiresOn)
		result = append(result, subscription)
	}
	return result, nil
}

func (s *SQLStore) SaveGlobalSubscription(subscription storemodels.GlobalSubscription) error {
	if _, err := s.getQueryBuilder().Delete("msteamssync_subscriptions").Where(sq.Eq{"type": subscription.Type}).Exec(); err != nil {
		return err
	}

	if _, err := s.getQueryBuilder().Insert("msteamssync_subscriptions").Columns("subscriptionID, type, secret, expiresOn").Values(subscription.SubscriptionID, subscription.Type, subscription.Secret, subscription.ExpiresOn.UnixMicro()).Exec(); err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) SaveChatSubscription(subscription storemodels.ChatSubscription) error {
	if _, err := s.getQueryBuilder().Delete("msteamssync_subscriptions").Where(sq.Eq{"msteamsUserID": subscription.UserID}).Exec(); err != nil {
		return err
	}

	if _, err := s.getQueryBuilder().Insert("msteamssync_subscriptions").Columns("subscriptionID, msTeamsUserID, type, secret, expiresOn").Values(subscription.SubscriptionID, subscription.UserID, subscriptionTypeUser, subscription.Secret, subscription.ExpiresOn.UnixMicro()).Exec(); err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) SaveChannelSubscription(subscription storemodels.ChannelSubscription) error {
	if _, err := s.getQueryBuilder().Delete("msteamssync_subscriptions").Where(sq.Eq{"msTeamsTeamID": subscription.TeamID, "msTeamsChannelID": subscription.ChannelID}).Exec(); err != nil {
		return err
	}

	if _, err := s.getQueryBuilder().Insert("msteamssync_subscriptions").Columns("subscriptionID, msTeamsTeamID, msTeamsChannelID, type, secret, expiresOn").Values(subscription.SubscriptionID, subscription.TeamID, subscription.ChannelID, subscriptionTypeChannel, subscription.Secret, subscription.ExpiresOn.UnixMicro()).Exec(); err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) UpdateSubscriptionExpiresOn(subscriptionID string, expiresOn time.Time) error {
	query := s.getQueryBuilder().Update("msteamssync_subscriptions").Set("expiresOn", expiresOn.UnixMicro()).Where(sq.Eq{"subscriptionID": subscriptionID})
	_, err := query.Exec()
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) DeleteSubscription(subscriptionID string) error {
	if _, err := s.getQueryBuilder().Delete("msteamssync_subscriptions").Where(sq.Eq{"subscriptionID": subscriptionID}).Exec(); err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) GetChannelSubscription(subscriptionID string) (*storemodels.ChannelSubscription, error) {
	row := s.getQueryBuilder().Select("subscriptionID, msTeamsChannelID, msTeamsTeamID, secret, expiresOn").From("msteamssync_subscriptions").Where(sq.Eq{"subscriptionID": subscriptionID, "type": subscriptionTypeChannel}).QueryRow()
	var subscription storemodels.ChannelSubscription
	var expiresOn int64
	if scanErr := row.Scan(&subscription.SubscriptionID, &subscription.ChannelID, &subscription.TeamID, &subscription.Secret, &expiresOn); scanErr != nil {
		return nil, scanErr
	}
	subscription.ExpiresOn = time.UnixMicro(expiresOn)
	return &subscription, nil
}

func (s *SQLStore) GetChatSubscription(subscriptionID string) (*storemodels.ChatSubscription, error) {
	row := s.getQueryBuilder().Select("subscriptionID, msTeamsUserID, secret, expiresOn").From("msteamssync_subscriptions").Where(sq.Eq{"subscriptionID": subscriptionID, "type": subscriptionTypeUser}).QueryRow()
	var subscription storemodels.ChatSubscription
	var expiresOn int64
	if scanErr := row.Scan(&subscription.SubscriptionID, &subscription.UserID, &subscription.Secret, &expiresOn); scanErr != nil {
		return nil, scanErr
	}
	subscription.ExpiresOn = time.UnixMicro(expiresOn)
	return &subscription, nil
}

func (s *SQLStore) GetGlobalSubscription(subscriptionID string) (*storemodels.GlobalSubscription, error) {
	row := s.getQueryBuilder().Select("subscriptionID, type, secret, expiresOn").From("msteamssync_subscriptions").Where(sq.Eq{"subscriptionID": subscriptionID, "type": subscriptionTypeAllChats}).QueryRow()
	var subscription storemodels.GlobalSubscription
	var expiresOn int64
	if scanErr := row.Scan(&subscription.SubscriptionID, &subscription.Type, &subscription.Secret, &expiresOn); scanErr != nil {
		return nil, scanErr
	}
	subscription.ExpiresOn = time.UnixMicro(expiresOn)
	return &subscription, nil
}

func (s *SQLStore) GetSubscriptionType(subscriptionID string) (string, error) {
	row := s.getQueryBuilder().Select("type").From("msteamssync_subscriptions").Where(sq.Eq{"subscriptionID": subscriptionID}).QueryRow()
	var subscriptionType string
	if scanErr := row.Scan(&subscriptionType); scanErr != nil {
		return "", scanErr
	}
	return subscriptionType, nil
}

func (s *SQLStore) StoreDMAndGMChannelPromptTime(channelID, userID string, timestamp time.Time) error {
	timeBytes, err := timestamp.MarshalJSON()
	if err != nil {
		return err
	}

	if err := s.api.KVSet(connectionPromptKey+channelID+"_"+userID, timeBytes); err != nil {
		return errors.New(err.Error())
	}

	return nil
}

func (s *SQLStore) GetDMAndGMChannelPromptTime(channelID, userID string) (time.Time, error) {
	var t time.Time
	data, err := s.api.KVGet(connectionPromptKey + channelID + "_" + userID)
	if err != nil {
		return t, errors.New(err.Error())
	}

	if err := t.UnmarshalJSON(data); err != nil {
		return t, err
	}

	return t, nil
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
