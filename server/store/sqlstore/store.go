package sqlstore

import (
	"crypto/sha512"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	connectionPromptKey          = "connect_"
	subscriptionRefreshTimeLimit = 5 * time.Minute
	maxLimitForLinks             = 100
	subscriptionTypeUser         = "user"
	subscriptionTypeChannel      = "channel"
	subscriptionTypeAllChats     = "allChats"
	oAuth2StateTimeToLive        = 300 // seconds
	oAuth2KeyPrefix              = "oauth2_"
	backgroundJobPrefix          = "background_job"
	usersTableName               = "msteamssync_users"
	linksTableName               = "msteamssync_links"
	postsTableName               = "msteamssync_posts"
	subscriptionsTableName       = "msteamssync_subscriptions"
	whitelistedUsersTableName    = "msteamssync_whitelisted_users"
	PGUniqueViolationErrorCode   = "23505" // See https://github.com/lib/pq/blob/master/error.go#L178
)

type SQLStore struct {
	api           plugin.API
	enabledTeams  func() []string
	encryptionKey func() []byte
	db            *sql.DB
}

func New(db *sql.DB, api plugin.API, enabledTeams func() []string, encryptionKey func() []byte) *SQLStore {
	return &SQLStore{
		db:            db,
		api:           api,
		enabledTeams:  enabledTeams,
		encryptionKey: encryptionKey,
	}
}

func (s *SQLStore) createTable(tableName, columnList string) error {
	if _, err := s.db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, columnList)); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) createIndex(tableName, indexName, columnList string) error {
	if _, err := s.db.Exec(fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", indexName, tableName, columnList)); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) addColumn(tableName, columnName, columnDefinition string) error {
	if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s", tableName, columnName, columnDefinition)); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) addPrimaryKey(tableName, columnList string) error {
	rows, err := s.db.Query(fmt.Sprintf("SELECT constraint_name from information_schema.table_constraints where table_name = '%s' and constraint_type='PRIMARY KEY'", tableName))
	if err != nil {
		return err
	}
	defer rows.Close()

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

	return nil
}

func (s *SQLStore) Init(remoteID string) error {
	if err := s.createTable(subscriptionsTableName, "subscriptionID VARCHAR(255) PRIMARY KEY, type VARCHAR(255), msTeamsTeamID VARCHAR(255), msTeamsChannelID VARCHAR(255), msTeamsUserID VARCHAR(255), secret VARCHAR(255), expiresOn BIGINT"); err != nil {
		return err
	}

	if err := s.createTable(linksTableName, "mmChannelID VARCHAR(255) PRIMARY KEY, mmTeamID VARCHAR(255), msTeamsChannelID VARCHAR(255), msTeamsTeamID VARCHAR(255), creator VARCHAR(255)"); err != nil {
		return err
	}

	if err := s.addColumn(linksTableName, "creator", "VARCHAR(255)"); err != nil {
		return err
	}

	if err := s.createTable(usersTableName, "mmUserID VARCHAR(255) PRIMARY KEY, msTeamsUserID VARCHAR(255), token TEXT"); err != nil {
		return err
	}

	if err := s.addPrimaryKey(usersTableName, "mmUserID, msTeamsUserID"); err != nil {
		return err
	}

	if err := s.createTable(postsTableName, "mmPostID VARCHAR(255) PRIMARY KEY, msTeamsPostID VARCHAR(255), msTeamsChannelID VARCHAR(255), msTeamsLastUpdateAt BIGINT"); err != nil {
		return err
	}

	if err := s.createIndex(linksTableName, "idx_msteamssync_links_msteamsteamid_msteamschannelid", "msTeamsTeamID, msTeamsChannelID"); err != nil {
		return err
	}

	if err := s.createIndex(usersTableName, "idx_msteamssync_users_msteamsuserid", "msTeamsUserID"); err != nil {
		return err
	}

	if err := s.createIndex(postsTableName, "idx_msteamssync_posts_msteamschannelid_msteamspostid", "msTeamsChannelID, msTeamsPostID"); err != nil {
		return err
	}

	if err := s.addColumn(subscriptionsTableName, "certificate", "TEXT"); err != nil {
		return err
	}

	if err := s.addColumn(subscriptionsTableName, "lastActivityAt", "BIGINT"); err != nil {
		return err
	}

	if err := s.createTable(whitelistedUsersTableName, "mmUserID VARCHAR(255) PRIMARY KEY"); err != nil {
		return err
	}

	return s.runMigrationRemoteID(remoteID)
}

func (s *SQLStore) ListChannelLinksWithNames() ([]*storemodels.ChannelLink, error) {
	query := s.getQueryBuilder().Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator, Teams.DisplayName, Channels.DisplayName").From(linksTableName).LeftJoin("Teams ON Teams.Id = msteamssync_links.mmTeamID").LeftJoin("Channels ON Channels.Id = msteamssync_links.mmChannelID").Limit(maxLimitForLinks)
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*storemodels.ChannelLink
	for rows.Next() {
		link := &storemodels.ChannelLink{}
		if err := rows.Scan(&link.MattermostChannelID, &link.MattermostTeamID, &link.MSTeamsChannel, &link.MSTeamsTeam, &link.Creator, &link.MattermostTeamName, &link.MattermostChannelName); err != nil {
			s.api.LogDebug("Unable to scan the result", "Error", err.Error())
			continue
		}

		links = append(links, link)
	}

	return links, nil
}

func (s *SQLStore) GetLinkByChannelID(channelID string) (*storemodels.ChannelLink, error) {
	query := s.getQueryBuilder().Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator").From(linksTableName).Where(sq.Eq{"mmChannelID": channelID})
	row := query.QueryRow()
	var link storemodels.ChannelLink
	err := row.Scan(&link.MattermostChannelID, &link.MattermostTeamID, &link.MSTeamsChannel, &link.MSTeamsTeam, &link.Creator)
	if err != nil {
		return nil, err
	}

	if !s.CheckEnabledTeamByTeamID(link.MattermostTeamID) {
		return nil, errors.New("link not enabled for this team")
	}
	return &link, nil
}

func (s *SQLStore) ListChannelLinks() ([]storemodels.ChannelLink, error) {
	rows, err := s.getQueryBuilder().Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator").From(linksTableName).Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	links := []storemodels.ChannelLink{}
	for rows.Next() {
		var link storemodels.ChannelLink
		err := rows.Scan(&link.MattermostChannelID, &link.MattermostTeamID, &link.MSTeamsChannel, &link.MSTeamsTeam, &link.Creator)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}

	return links, nil
}

func (s *SQLStore) GetLinkByMSTeamsChannelID(teamID, channelID string) (*storemodels.ChannelLink, error) {
	query := s.getQueryBuilder().Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator").From(linksTableName).Where(sq.Eq{"msTeamsTeamID": teamID, "msTeamsChannelID": channelID})
	row := query.QueryRow()
	var link storemodels.ChannelLink
	err := row.Scan(&link.MattermostChannelID, &link.MattermostTeamID, &link.MSTeamsChannel, &link.MSTeamsTeam, &link.Creator)
	if err != nil {
		return nil, err
	}
	if !s.CheckEnabledTeamByTeamID(link.MattermostTeamID) {
		return nil, errors.New("link not enabled for this team")
	}
	return &link, nil
}

func (s *SQLStore) DeleteLinkByChannelID(channelID string) error {
	query := s.getQueryBuilder().Delete(linksTableName).Where(sq.Eq{"mmChannelID": channelID})
	_, err := query.Exec()
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) StoreChannelLink(link *storemodels.ChannelLink) error {
	query := s.getQueryBuilder().Insert(linksTableName).Columns("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator").Values(link.MattermostChannelID, link.MattermostTeamID, link.MSTeamsChannel, link.MSTeamsTeam, link.Creator)
	_, err := query.Exec()
	if err != nil {
		return err
	}
	if !s.CheckEnabledTeamByTeamID(link.MattermostTeamID) {
		return errors.New("link not enabled for this team")
	}
	return nil
}

func (s *SQLStore) TeamsToMattermostUserID(userID string) (string, error) {
	query := s.getQueryBuilder().Select("mmUserID").From(usersTableName).Where(sq.Eq{"msTeamsUserID": userID})
	row := query.QueryRow()
	var mmUserID string
	err := row.Scan(&mmUserID)
	if err != nil {
		return "", err
	}
	return mmUserID, nil
}

func (s *SQLStore) MattermostToTeamsUserID(userID string) (string, error) {
	query := s.getQueryBuilder().Select("msTeamsUserID").From(usersTableName).Where(sq.Eq{"mmUserID": userID})
	row := query.QueryRow()
	var msTeamsUserID string
	err := row.Scan(&msTeamsUserID)
	if err != nil {
		return "", err
	}
	return msTeamsUserID, nil
}

func (s *SQLStore) GetPostInfoByMSTeamsID(chatID string, postID string) (*storemodels.PostInfo, error) {
	query := s.getQueryBuilder().Select("mmPostID, msTeamsLastUpdateAt").From(postsTableName).Where(sq.Eq{"msTeamsPostID": postID, "msTeamsChannelID": chatID}).Suffix("FOR UPDATE")
	row := query.QueryRow()
	var lastUpdateAt int64
	postInfo := storemodels.PostInfo{
		MSTeamsID:      postID,
		MSTeamsChannel: chatID,
	}

	if err := row.Scan(&postInfo.MattermostID, &lastUpdateAt); err != nil {
		return nil, err
	}
	postInfo.MSTeamsLastUpdateAt = time.UnixMicro(lastUpdateAt)
	return &postInfo, nil
}

func (s *SQLStore) GetPostInfoByMattermostID(postID string) (*storemodels.PostInfo, error) {
	query := s.getQueryBuilder().Select("msTeamsPostID, msTeamsChannelID, msTeamsLastUpdateAt").From(postsTableName).Where(sq.Eq{"mmPostID": postID}).Suffix("FOR UPDATE")
	row := query.QueryRow()
	var lastUpdateAt int64
	postInfo := storemodels.PostInfo{
		MattermostID: postID,
	}

	if err := row.Scan(&postInfo.MSTeamsID, &postInfo.MSTeamsChannel, &lastUpdateAt); err != nil {
		return nil, err
	}

	postInfo.MSTeamsLastUpdateAt = time.UnixMicro(lastUpdateAt)
	return &postInfo, nil
}

func (s *SQLStore) SetPostLastUpdateAtByMattermostID(postID string, lastUpdateAt time.Time) error {
	query := s.getQueryBuilder().Update(postsTableName).Set("msTeamsLastUpdateAt", lastUpdateAt.UnixMicro()).Where(sq.Eq{"mmPostID": postID})
	if _, err := query.Exec(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) SetPostLastUpdateAtByMSTeamsID(msTeamsPostID string, lastUpdateAt time.Time) error {
	query := s.getQueryBuilder().Update(postsTableName).Set("msTeamsLastUpdateAt", lastUpdateAt.UnixMicro()).Where(sq.Eq{"msTeamsPostID": msTeamsPostID})
	if _, err := query.Exec(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) LinkPosts(postInfo storemodels.PostInfo) error {
	query := s.getQueryBuilder().Insert(postsTableName).Columns("mmPostID, msTeamsPostID, msTeamsChannelID, msTeamsLastUpdateAt").Values(
		postInfo.MattermostID,
		postInfo.MSTeamsID,
		postInfo.MSTeamsChannel,
		postInfo.MSTeamsLastUpdateAt.UnixMicro(),
	).Suffix("ON CONFLICT (mmPostID) DO UPDATE SET msTeamsPostID = EXCLUDED.msTeamsPostID, msTeamsChannelID = EXCLUDED.msTeamsChannelID, msTeamsLastUpdateAt = EXCLUDED.msTeamsLastUpdateAt")
	if _, err := query.Exec(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) GetTokenForMattermostUser(userID string) (*oauth2.Token, error) {
	query := s.getQueryBuilder().Select("token").From(usersTableName).Where(sq.Eq{"mmUserID": userID}).Where(sq.NotEq{"token": ""})
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
	query := s.getQueryBuilder().Select("token").From(usersTableName).Where(sq.Eq{"msTeamsUserID": userID}).Where(sq.NotEq{"token": ""})
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

	if err := s.DeleteUserInfo(userID); err != nil {
		return err
	}

	if _, err := s.getQueryBuilder().Insert(usersTableName).Columns("mmUserID, msTeamsUserID, token").Values(userID, msTeamsUserID, encryptedToken).Suffix("ON CONFLICT (mmUserID, msTeamsUserID) DO UPDATE SET token = EXCLUDED.token").Exec(); err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) DeleteUserInfo(mmUserID string) error {
	if _, err := s.getQueryBuilder().Delete(usersTableName).Where(sq.Eq{"mmUserID": mmUserID}).Exec(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) ListChatSubscriptionsToCheck() ([]storemodels.ChatSubscription, error) {
	expireTime := time.Now().Add(subscriptionRefreshTimeLimit).UnixMicro()
	query := s.getQueryBuilder().Select("subscriptionID, msTeamsUserID, secret, expiresOn, certificate").From(subscriptionsTableName).Where(sq.Eq{"type": subscriptionTypeUser}).Where(sq.Lt{"expiresOn": expireTime})
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []storemodels.ChatSubscription{}
	for rows.Next() {
		var subscription storemodels.ChatSubscription
		var expiresOn int64
		var certificate *string
		if scanErr := rows.Scan(&subscription.SubscriptionID, &subscription.UserID, &subscription.Secret, &expiresOn, &certificate); scanErr != nil {
			return nil, scanErr
		}
		if certificate != nil {
			subscription.Certificate = *certificate
		}
		subscription.ExpiresOn = time.UnixMicro(expiresOn)
		result = append(result, subscription)
	}
	return result, nil
}

func (s *SQLStore) ListChannelSubscriptions() ([]*storemodels.ChannelSubscription, error) {
	query := s.getQueryBuilder().Select("subscriptionID, msTeamsChannelID, msTeamsTeamID, secret, expiresOn, certificate").From(subscriptionsTableName).Where(sq.Eq{"type": subscriptionTypeChannel})
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*storemodels.ChannelSubscription{}
	for rows.Next() {
		var subscription storemodels.ChannelSubscription
		var expiresOn int64
		var certificate *string
		if scanErr := rows.Scan(&subscription.SubscriptionID, &subscription.ChannelID, &subscription.TeamID, &subscription.Secret, &expiresOn, &certificate); scanErr != nil {
			return nil, scanErr
		}
		if certificate != nil {
			subscription.Certificate = *certificate
		}

		subscription.ExpiresOn = time.UnixMicro(expiresOn)
		result = append(result, &subscription)
	}
	return result, nil
}

func (s *SQLStore) ListChannelSubscriptionsToRefresh(certificate string) ([]*storemodels.ChannelSubscription, error) {
	expireTime := time.Now().Add(subscriptionRefreshTimeLimit).UnixMicro()
	query := s.getQueryBuilder().
		Select("subscriptionID, msTeamsChannelID, msTeamsTeamID, secret, expiresOn, certificate").
		From(subscriptionsTableName).
		Where(sq.Eq{"type": subscriptionTypeChannel}).
		Where(sq.Or{sq.NotEq{"certificate": certificate}, sq.Lt{"expiresOn": expireTime}})
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*storemodels.ChannelSubscription{}
	for rows.Next() {
		var subscription storemodels.ChannelSubscription
		var expiresOn int64
		var cert *string
		if scanErr := rows.Scan(&subscription.SubscriptionID, &subscription.ChannelID, &subscription.TeamID, &subscription.Secret, &expiresOn, &cert); scanErr != nil {
			return nil, scanErr
		}
		if cert != nil {
			subscription.Certificate = *cert
		}
		subscription.ExpiresOn = time.UnixMicro(expiresOn)
		result = append(result, &subscription)
	}
	return result, nil
}

func (s *SQLStore) ListGlobalSubscriptions() ([]*storemodels.GlobalSubscription, error) {
	query := s.getQueryBuilder().Select("subscriptionID, type, secret, expiresOn, certificate").From(subscriptionsTableName).Where(sq.Eq{"type": subscriptionTypeAllChats})
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*storemodels.GlobalSubscription{}
	for rows.Next() {
		var subscription storemodels.GlobalSubscription
		var expiresOn int64
		var certificate *string
		if scanErr := rows.Scan(&subscription.SubscriptionID, &subscription.Type, &subscription.Secret, &expiresOn, &certificate); scanErr != nil {
			return nil, scanErr
		}
		if certificate != nil {
			subscription.Certificate = *certificate
		}

		subscription.ExpiresOn = time.UnixMicro(expiresOn)
		result = append(result, &subscription)
	}
	return result, nil
}

func (s *SQLStore) ListGlobalSubscriptionsToRefresh(certificate string) ([]*storemodels.GlobalSubscription, error) {
	expireTime := time.Now().Add(subscriptionRefreshTimeLimit).UnixMicro()
	query := s.getQueryBuilder().
		Select("subscriptionID, type, secret, expiresOn, certificate").
		From(subscriptionsTableName).
		Where(sq.Eq{"type": subscriptionTypeAllChats}).
		Where(sq.Or{sq.NotEq{"certificate": certificate}, sq.Lt{"expiresOn": expireTime}})
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*storemodels.GlobalSubscription{}
	for rows.Next() {
		var subscription storemodels.GlobalSubscription
		var expiresOn int64
		var certificate *string
		if scanErr := rows.Scan(&subscription.SubscriptionID, &subscription.Type, &subscription.Secret, &expiresOn, &certificate); scanErr != nil {
			return nil, scanErr
		}
		if certificate != nil {
			subscription.Certificate = *certificate
		}
		subscription.ExpiresOn = time.UnixMicro(expiresOn)
		result = append(result, &subscription)
	}
	return result, nil
}

func (s *SQLStore) SaveGlobalSubscription(subscription storemodels.GlobalSubscription) error {
	if _, err := s.getQueryBuilder().Delete(subscriptionsTableName).Where(sq.Eq{"type": subscription.Type}).Exec(); err != nil {
		return err
	}

	if _, err := s.getQueryBuilder().Insert(subscriptionsTableName).Columns("subscriptionID, type, secret, expiresOn, certificate").Values(subscription.SubscriptionID, subscription.Type, subscription.Secret, subscription.ExpiresOn.UnixMicro(), subscription.Certificate).Exec(); err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) SaveChatSubscription(subscription storemodels.ChatSubscription) error {
	if _, err := s.getQueryBuilder().Delete(subscriptionsTableName).Where(sq.Eq{"msteamsUserID": subscription.UserID}).Exec(); err != nil {
		return err
	}

	if _, err := s.getQueryBuilder().Insert(subscriptionsTableName).Columns("subscriptionID, msTeamsUserID, type, secret, expiresOn, certificate").Values(subscription.SubscriptionID, subscription.UserID, subscriptionTypeUser, subscription.Secret, subscription.ExpiresOn.UnixMicro(), subscription.Certificate).Exec(); err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) SaveChannelSubscription(subscription storemodels.ChannelSubscription) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := s.getQueryBuilder().Delete(subscriptionsTableName).Where(sq.Eq{"msTeamsTeamID": subscription.TeamID, "msTeamsChannelID": subscription.ChannelID}).RunWith(tx).Exec(); err != nil {
		return err
	}

	if _, err := s.getQueryBuilder().Insert(subscriptionsTableName).Columns("subscriptionID, msTeamsTeamID, msTeamsChannelID, type, secret, expiresOn, certificate").Values(subscription.SubscriptionID, subscription.TeamID, subscription.ChannelID, subscriptionTypeChannel, subscription.Secret, subscription.ExpiresOn.UnixMicro(), subscription.Certificate).RunWith(tx).Exec(); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLStore) UpdateSubscriptionExpiresOn(subscriptionID string, expiresOn time.Time) error {
	query := s.getQueryBuilder().Update(subscriptionsTableName).Set("expiresOn", expiresOn.UnixMicro()).Where(sq.Eq{"subscriptionID": subscriptionID})
	_, err := query.Exec()
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) UpdateSubscriptionLastActivityAt(subscriptionID string, lastActivityAt time.Time) error {
	query := s.getQueryBuilder().
		Update(subscriptionsTableName).
		Set("lastActivityAt", lastActivityAt.UnixMicro()).
		Where(sq.And{
			sq.Eq{"subscriptionID": subscriptionID},
			sq.Or{sq.Lt{"lastActivityAt": lastActivityAt.UnixMicro()}, sq.Eq{"lastActivityAt": nil}},
		})
	_, err := query.Exec()
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) GetSubscriptionsLastActivityAt() (map[string]time.Time, error) {
	query := s.getQueryBuilder().
		Select("subscriptionID, lastActivityAt").
		From(subscriptionsTableName).
		Where(
			sq.NotEq{"lastActivityAt": nil},
			sq.NotEq{"lastActivityAt": 0},
		)
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]time.Time{}
	for rows.Next() {
		var lastActivityAt int64
		var subscriptionID string
		if scanErr := rows.Scan(&subscriptionID, &lastActivityAt); scanErr != nil {
			return nil, scanErr
		}
		result[subscriptionID] = time.UnixMicro(lastActivityAt)
	}
	return result, nil
}

func (s *SQLStore) DeleteSubscription(subscriptionID string) error {
	if _, err := s.getQueryBuilder().Delete(subscriptionsTableName).Where(sq.Eq{"subscriptionID": subscriptionID}).Exec(); err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) GetChannelSubscription(subscriptionID string) (*storemodels.ChannelSubscription, error) {
	row := s.getQueryBuilder().Select("subscriptionID, msTeamsChannelID, msTeamsTeamID, secret, expiresOn, certificate").From(subscriptionsTableName).Where(sq.Eq{"subscriptionID": subscriptionID, "type": subscriptionTypeChannel}).Suffix("FOR UPDATE").QueryRow()
	var subscription storemodels.ChannelSubscription
	var expiresOn int64
	var certificate *string
	if err := row.Scan(&subscription.SubscriptionID, &subscription.ChannelID, &subscription.TeamID, &subscription.Secret, &expiresOn, &certificate); err != nil {
		return nil, err
	}
	if certificate != nil {
		subscription.Certificate = *certificate
	}
	subscription.ExpiresOn = time.UnixMicro(expiresOn)
	return &subscription, nil
}

func (s *SQLStore) GetChannelSubscriptionByTeamsChannelID(teamsChannelID string) (*storemodels.ChannelSubscription, error) {
	row := s.getQueryBuilder().Select("subscriptionID").From(subscriptionsTableName).Where(sq.Eq{"msTeamsChannelID": teamsChannelID, "type": subscriptionTypeChannel}).QueryRow()
	var subscription storemodels.ChannelSubscription
	if scanErr := row.Scan(&subscription.SubscriptionID); scanErr != nil {
		return nil, scanErr
	}
	return &subscription, nil
}

func (s *SQLStore) GetChatSubscription(subscriptionID string) (*storemodels.ChatSubscription, error) {
	row := s.getQueryBuilder().Select("subscriptionID, msTeamsUserID, secret, expiresOn, certificate").From(subscriptionsTableName).Where(sq.Eq{"subscriptionID": subscriptionID, "type": subscriptionTypeUser}).QueryRow()
	var subscription storemodels.ChatSubscription
	var expiresOn int64
	var certificate *string
	if scanErr := row.Scan(&subscription.SubscriptionID, &subscription.UserID, &subscription.Secret, &expiresOn, &certificate); scanErr != nil {
		return nil, scanErr
	}
	if certificate != nil {
		subscription.Certificate = *certificate
	}
	subscription.ExpiresOn = time.UnixMicro(expiresOn)
	return &subscription, nil
}

func (s *SQLStore) GetGlobalSubscription(subscriptionID string) (*storemodels.GlobalSubscription, error) {
	row := s.getQueryBuilder().Select("subscriptionID, type, secret, expiresOn, certificate").From(subscriptionsTableName).Where(sq.Eq{"subscriptionID": subscriptionID, "type": subscriptionTypeAllChats}).QueryRow()
	var subscription storemodels.GlobalSubscription
	var expiresOn int64
	var certificate *string
	if scanErr := row.Scan(&subscription.SubscriptionID, &subscription.Type, &subscription.Secret, &expiresOn, &certificate); scanErr != nil {
		return nil, scanErr
	}
	if certificate != nil {
		subscription.Certificate = *certificate
	}
	subscription.ExpiresOn = time.UnixMicro(expiresOn)
	return &subscription, nil
}

func (s *SQLStore) GetSubscriptionType(subscriptionID string) (string, error) {
	row := s.getQueryBuilder().Select("type").From(subscriptionsTableName).Where(sq.Eq{"subscriptionID": subscriptionID}).QueryRow()
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

func (s *SQLStore) DeleteDMAndGMChannelPromptTime(userID string) error {
	var userKeys []string
	page := 0
	perPage := 100
	for {
		keys, err := s.api.KVList(page, perPage)
		if err != nil {
			return errors.New(err.Error())
		}

		for _, key := range keys {
			if strings.HasPrefix(key, connectionPromptKey) && strings.Contains(key, userID) {
				userKeys = append(userKeys, key)
			}
		}

		if len(keys) < perPage {
			break
		}
		page++
	}

	for _, key := range userKeys {
		if err := s.api.KVDelete(key); err != nil {
			return errors.New(err.Error())
		}
	}

	return nil
}

func (s *SQLStore) RecoverPost(postID string) error {
	query := s.getQueryBuilder().Update("Posts").Set("DeleteAt", 0).Where(sq.Eq{"Id": postID}, sq.NotEq{"DeleteAt": 0})
	if _, err := query.Exec(); err != nil {
		return err
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
	return sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(s.db)
}

func (s *SQLStore) VerifyOAuth2State(state string) error {
	key := hashKey(oAuth2KeyPrefix, state)
	data, appErr := s.api.KVGet(key)
	if appErr != nil {
		return errors.New(appErr.Message)
	}

	if data == nil {
		return errors.New("authentication attempt expired, please try again")
	}

	if string(data) != state {
		return errors.New("invalid oauth state, please try again")
	}

	return nil
}

func (s *SQLStore) StoreOAuth2State(state string) error {
	key := hashKey(oAuth2KeyPrefix, state)
	if err := s.api.KVSetWithExpiry(key, []byte(state), oAuth2StateTimeToLive); err != nil {
		return errors.New(err.Message)
	}

	return nil
}

func (s *SQLStore) GetStats() (*storemodels.Stats, error) {
	query := s.getQueryBuilder().Select("count(mmChannelID)").From(linksTableName)
	row := query.QueryRow()
	var linkedChannels int64
	if err := row.Scan(&linkedChannels); err != nil {
		return nil, err
	}

	query = s.getQueryBuilder().Select("count(mmUserID)").From(usersTableName).Where(sq.NotEq{"token": ""}).Where(sq.NotEq{"token": nil})
	row = query.QueryRow()
	var connectedUsers int64
	if err := row.Scan(&connectedUsers); err != nil {
		return nil, err
	}

	query = s.getQueryBuilder().Select("count(id)").From("users").Where(sq.NotEq{"RemoteId": ""}).Where(sq.Like{"Username": "msteams_%"}).Where(sq.Eq{"DeleteAt": 0})
	row = query.QueryRow()
	var syntheticUsers int64
	if err := row.Scan(&syntheticUsers); err != nil {
		return nil, err
	}

	return &storemodels.Stats{
		LinkedChannels: linkedChannels,
		ConnectedUsers: connectedUsers,
		SyntheticUsers: syntheticUsers,
	}, nil
}

func (s *SQLStore) GetConnectedUsers(page, perPage int) ([]*storemodels.ConnectedUser, error) {
	query := s.getQueryBuilder().Select("mmuserid, msteamsuserid, Users.FirstName, Users.LastName, Users.Email").From(usersTableName).LeftJoin("Users ON Users.Id = msteamssync_users.mmuserid").Where(sq.NotEq{"token": ""}).OrderBy("Users.FirstName").Offset(uint64(page * perPage)).Limit(uint64(perPage))
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var connectedUsers []*storemodels.ConnectedUser
	for rows.Next() {
		connectedUser := &storemodels.ConnectedUser{}
		if err := rows.Scan(&connectedUser.MattermostUserID, &connectedUser.TeamsUserID, &connectedUser.FirstName, &connectedUser.LastName, &connectedUser.Email); err != nil {
			s.api.LogDebug("Unable to scan the result", "Error", err.Error())
			continue
		}

		connectedUsers = append(connectedUsers, connectedUser)
	}

	return connectedUsers, nil
}

func (s *SQLStore) PrefillWhitelist() error {
	page := 0
	perPage := 100
	for {
		query := s.getQueryBuilder().Select("mmuserid").From(usersTableName).Where(sq.NotEq{"token": ""}).Offset(uint64(page * perPage)).Limit(uint64(perPage))
		rows, err := query.Query()
		if err != nil {
			return err
		}

		count := 0
		for rows.Next() {
			count++
			var connectedUserID string
			if err := rows.Scan(&connectedUserID); err != nil {
				s.api.LogDebug("Unable to scan the result", "Error", err.Error())
				continue
			}

			if err := s.StoreUserInWhitelist(connectedUserID); err != nil {
				s.api.LogDebug("Unable to store user in whitelist", "UserID", connectedUserID, "Error", err.Error())
			}
		}

		rows.Close()
		if count < perPage {
			break
		}

		page++
	}

	return nil
}

func (s *SQLStore) GetSizeOfWhitelist() (int, error) {
	query := s.getQueryBuilder().Select("count(*)").From(whitelistedUsersTableName)
	rows, err := query.Query()
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var result int
	if rows.Next() {
		if scanErr := rows.Scan(&result); scanErr != nil {
			return 0, scanErr
		}
	}

	return result, nil
}

func (s *SQLStore) StoreUserInWhitelist(userID string) error {
	query := s.getQueryBuilder().Insert(whitelistedUsersTableName).Columns("mmUserID").Values(userID)
	if _, err := query.Exec(); err != nil {
		if isDuplicate(err) {
			s.api.LogDebug("UserID already present in whitelist", "UserID", userID)
			return nil
		}

		return err
	}

	return nil
}

func (s *SQLStore) IsUserPresentInWhitelist(userID string) (bool, error) {
	query := s.getQueryBuilder().Select("mmUserID").From(whitelistedUsersTableName).Where(sq.Eq{"mmUserID": userID})
	rows, err := query.Query()
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var result string
	if rows.Next() {
		if scanErr := rows.Scan(&result); scanErr != nil {
			return false, scanErr
		}
	}

	return result != "", nil
}

func hashKey(prefix, hashableKey string) string {
	if hashableKey == "" {
		return prefix
	}

	h := sha512.New()
	_, _ = h.Write([]byte(hashableKey))
	return fmt.Sprintf("%s%x", prefix, h.Sum(nil))
}

// isDuplicate checks whether an error is a duplicate key error, which comes when processes are competing on creating the same
// tables in the database.
func isDuplicate(err error) bool {
	var pqErr *pq.Error
	if errors.As(errors.Cause(err), &pqErr) {
		if pqErr.Code == PGUniqueViolationErrorCode {
			return true
		}
	}

	return false
}
