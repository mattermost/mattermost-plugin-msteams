package sqlstore

import (
	"crypto/sha512"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	connectionPromptKey             = "connect_"
	subscriptionRefreshTimeLimit    = 5 * time.Minute
	maxLimitForLinks                = 100
	setWhitelistFailureThreshold    = 0
	subscriptionTypeUser            = "user"
	subscriptionTypeChannel         = "channel"
	subscriptionTypeAllChats        = "allChats"
	oAuth2StateTimeToLive           = 300 // seconds
	oAuth2KeyPrefix                 = "oauth2_"
	backgroundJobPrefix             = "background_job"
	usersTableName                  = "msteamssync_users"
	linksTableName                  = "msteamssync_links"
	postsTableName                  = "msteamssync_posts"
	subscriptionsTableName          = "msteamssync_subscriptions"
	whitelistedUsersLegacyTableName = "msteamssync_whitelisted_users" // LEGACY-UNUSED
	whitelistTableName              = "msteamssync_whitelist"
	invitedUsersTableName           = "msteamssync_invited_users"
	PGUniqueViolationErrorCode      = "23505" // See https://github.com/lib/pq/blob/master/error.go#L178
)

type SQLStore struct {
	api           plugin.API
	enabledTeams  func() []string
	encryptionKey func() []byte
	db            *sql.DB
	replica       *sql.DB
}

func New(db, replica *sql.DB, api plugin.API, enabledTeams func() []string, encryptionKey func() []byte) *SQLStore {
	return &SQLStore{
		db:      db,
		replica: replica,
		api:     api,

		enabledTeams:  enabledTeams,
		encryptionKey: encryptionKey,
	}
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

	if err := s.createTable(invitedUsersTableName, "mmUserID VARCHAR(255) PRIMARY KEY"); err != nil {
		return err
	}

	if err := s.addColumn(invitedUsersTableName, "invitePendingSince", "BIGINT"); err != nil {
		return err
	}

	if err := s.addColumn(invitedUsersTableName, "inviteLastSentAt", "BIGINT"); err != nil {
		return err
	}

	if err := s.addColumn(usersTableName, "lastConnectAt", "BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	if err := s.addColumn(usersTableName, "lastDisconnectAt", "BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	if err := s.createTable(whitelistTableName, "mmUserID VARCHAR(255) PRIMARY KEY"); err != nil {
		return err
	}

	if remoteID != "" {
		if err := s.runMigrationRemoteID(remoteID); err != nil {
			return err
		}

		if err := s.runSetEmailVerifiedToTrueForRemoteUsers(remoteID); err != nil {
			return err
		}
	}

	exist, err := s.indexExist(usersTableName, "idx_msteamssync_users_msteamsuserid_unq")
	if err != nil {
		return err
	}
	if !exist {
		// dedup entries with multiples ms teams id
		if err := s.runMSTeamUserIDDedup(); err != nil {
			return err
		}

		if err := s.createMSTeamsUserIDUniqueIndex(); err != nil {
			return err
		}
	}

	if err := s.ensureMigrationWhitelistedUsers(); err != nil {
		return err
	}

	if err := s.addColumn(usersTableName, "LastChatSentAt", "BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	if err := s.addColumn(usersTableName, "LastChatReceivedAt", "BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	return nil
}

//db:withReplica
func (s *SQLStore) listChannelLinksWithNames(db sq.BaseRunner) ([]*storemodels.ChannelLink, error) {
	query := s.getQueryBuilder(db).Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator, Teams.DisplayName, Channels.DisplayName").From(linksTableName).LeftJoin("Teams ON Teams.Id = msteamssync_links.mmTeamID").LeftJoin("Channels ON Channels.Id = msteamssync_links.mmChannelID").Limit(maxLimitForLinks)
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

//db:withReplica
func (s *SQLStore) getLinkByChannelID(db sq.BaseRunner, channelID string) (*storemodels.ChannelLink, error) {
	query := s.getQueryBuilder(db).Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator").From(linksTableName).Where(sq.Eq{"mmChannelID": channelID})
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

//db:withReplica
func (s *SQLStore) listChannelLinks(db sq.BaseRunner) ([]storemodels.ChannelLink, error) {
	rows, err := s.getQueryBuilder(db).Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator").From(linksTableName).Query()
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

//db:withReplica
func (s *SQLStore) getLinkByMSTeamsChannelID(db sq.BaseRunner, teamID, channelID string) (*storemodels.ChannelLink, error) {
	query := s.getQueryBuilder(db).Select("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator").From(linksTableName).Where(sq.Eq{"msTeamsTeamID": teamID, "msTeamsChannelID": channelID})
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

func (s *SQLStore) deleteLinkByChannelID(db sq.BaseRunner, channelID string) error {
	query := s.getQueryBuilder(db).Delete(linksTableName).Where(sq.Eq{"mmChannelID": channelID})
	_, err := query.Exec()
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) storeChannelLink(db sq.BaseRunner, link *storemodels.ChannelLink) error {
	query := s.getQueryBuilder(db).Insert(linksTableName).Columns("mmChannelID, mmTeamID, msTeamsChannelID, msTeamsTeamID, creator").Values(link.MattermostChannelID, link.MattermostTeamID, link.MSTeamsChannel, link.MSTeamsTeam, link.Creator)
	_, err := query.Exec()
	if err != nil {
		return err
	}
	if !s.CheckEnabledTeamByTeamID(link.MattermostTeamID) {
		return errors.New("link not enabled for this team")
	}
	return nil
}

//db:withReplica
func (s *SQLStore) teamsToMattermostUserID(db sq.BaseRunner, userID string) (string, error) {
	query := s.getQueryBuilder(db).Select("mmUserID").From(usersTableName).Where(sq.Eq{"msTeamsUserID": userID})
	row := query.QueryRow()
	var mmUserID string
	err := row.Scan(&mmUserID)
	if err != nil {
		return "", err
	}
	return mmUserID, nil
}

//db:withReplica
func (s *SQLStore) mattermostToTeamsUserID(db sq.BaseRunner, userID string) (string, error) {
	query := s.getQueryBuilder(db).Select("msTeamsUserID").From(usersTableName).Where(sq.Eq{"mmUserID": userID})
	row := query.QueryRow()
	var msTeamsUserID string
	err := row.Scan(&msTeamsUserID)
	if err != nil {
		return "", err
	}
	return msTeamsUserID, nil
}

//db:withReplica
func (s *SQLStore) getPostInfoByMSTeamsID(db sq.BaseRunner, chatID string, postID string) (*storemodels.PostInfo, error) {
	query := s.getQueryBuilder(db).Select("mmPostID, msTeamsLastUpdateAt").From(postsTableName).Where(sq.Eq{"msTeamsPostID": postID, "msTeamsChannelID": chatID}).Suffix("FOR UPDATE")
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

//db:withReplica
func (s *SQLStore) getPostInfoByMattermostID(db sq.BaseRunner, postID string) (*storemodels.PostInfo, error) {
	query := s.getQueryBuilder(db).Select("msTeamsPostID, msTeamsChannelID, msTeamsLastUpdateAt").From(postsTableName).Where(sq.Eq{"mmPostID": postID}).Suffix("FOR UPDATE")
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

func (s *SQLStore) setPostLastUpdateAtByMattermostID(db sq.BaseRunner, postID string, lastUpdateAt time.Time) error {
	query := s.getQueryBuilder(db).Update(postsTableName).Set("msTeamsLastUpdateAt", lastUpdateAt.UnixMicro()).Where(sq.Eq{"mmPostID": postID})
	if _, err := query.Exec(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) setPostLastUpdateAtByMSTeamsID(db sq.BaseRunner, msTeamsPostID string, lastUpdateAt time.Time) error {
	query := s.getQueryBuilder(db).Update(postsTableName).Set("msTeamsLastUpdateAt", lastUpdateAt.UnixMicro()).Where(sq.Eq{"msTeamsPostID": msTeamsPostID})
	if _, err := query.Exec(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) linkPosts(db sq.BaseRunner, postInfo storemodels.PostInfo) error {
	query := s.getQueryBuilder(db).Insert(postsTableName).Columns("mmPostID, msTeamsPostID, msTeamsChannelID, msTeamsLastUpdateAt").Values(
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

//db:withReplica
func (s *SQLStore) getTokenForMattermostUser(db sq.BaseRunner, userID string) (*oauth2.Token, error) {
	query := s.getQueryBuilder(db).Select("token").From(usersTableName).Where(sq.Eq{"mmUserID": userID}).Where(sq.NotEq{"token": ""})
	row := query.QueryRow()
	var encryptedToken string
	err := row.Scan(&encryptedToken)
	if err != nil {
		return nil, err
	}

	if encryptedToken == "" {
		return nil, nil
	}

	tokendata, err := decrypt(s.encryptionKey(), encryptedToken)
	if err != nil {
		return nil, err
	}

	if tokendata == "" {
		return nil, nil
	}

	var token oauth2.Token
	err = json.Unmarshal([]byte(tokendata), &token)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

//db:withReplica
func (s *SQLStore) getTokenForMSTeamsUser(db sq.BaseRunner, userID string) (*oauth2.Token, error) {
	query := s.getQueryBuilder(db).Select("token").From(usersTableName).Where(sq.Eq{"msTeamsUserID": userID}).Where(sq.NotEq{"token": ""})
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

func (s *SQLStore) UserHasConnected(mmUserID string) (bool, error) {
	connectStatus, err := s.GetUserConnectStatus(mmUserID)

	if err != nil {
		return false, err
	}

	return !connectStatus.LastConnectAt.IsZero(), nil
}

//db:withReplica
func (s *SQLStore) getUserConnectStatus(db sq.BaseRunner, mmUserID string) (*storemodels.UserConnectStatus, error) {
	query := s.getQueryBuilder(db).
		Select("mmUserID", "token", "lastConnectAt", "lastDisconnectAt").
		From(usersTableName).
		Where(sq.Eq{"mmUserID": mmUserID})

	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &storemodels.UserConnectStatus{}
	if rows.Next() {
		var encryptedToken string
		var lastConnectAt int64
		var lastDisconnectAt int64

		if scanErr := rows.Scan(&result.ID, &encryptedToken, &lastConnectAt, &lastDisconnectAt); scanErr != nil {
			return nil, scanErr
		}

		if encryptedToken != "" {
			result.Connected = true
		}

		if lastConnectAt != 0 {
			result.LastConnectAt = time.UnixMicro(lastConnectAt)
		}

		if lastDisconnectAt != 0 {
			result.LastDisconnectAt = time.UnixMicro(lastDisconnectAt)
		}
	}

	return result, nil
}

func computeStatusTimes(status *storemodels.UserConnectStatus, nextIsConnected bool) (int64, int64, error) {
	var lastConnectAt int64
	var lastDisconnectAt int64

	now := time.Now()

	if nextIsConnected {
		// connected
		lastConnectAt = now.UnixMicro() // bump always if new token

		if !status.LastDisconnectAt.IsZero() {
			lastDisconnectAt = status.LastDisconnectAt.UnixMicro() // no change, pass-through
		}
	} else {
		if !status.LastConnectAt.IsZero() {
			lastConnectAt = status.LastConnectAt.UnixMicro() // pass-through
		}

		if status.Connected {
			lastDisconnectAt = now.UnixMicro() // bump only on actual disconnect
		} else if !status.LastDisconnectAt.IsZero() {
			lastDisconnectAt = status.LastDisconnectAt.UnixMicro() // no change, pass-through
		}
	}

	return lastConnectAt, lastDisconnectAt, nil
}

//db:withTransaction
func (s *SQLStore) setUserInfo(db sq.BaseRunner, userID string, msTeamsUserID string, token *oauth2.Token) error {
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

	currentConnectStatus, err := s.getUserConnectStatus(db, userID)
	if err != nil {
		return err
	}

	lastConnectAt, lastDisconnectAt, err := computeStatusTimes(currentConnectStatus, encryptedToken != "")
	if err != nil {
		return err
	}

	if err := s.deleteUserInfo(db, userID); err != nil {
		return err
	}

	if _, err := s.getQueryBuilder(db).Insert(usersTableName).Columns("mmUserID, msTeamsUserID, token, lastConnectAt, lastDisconnectAt").Values(userID, msTeamsUserID, encryptedToken, lastConnectAt, lastDisconnectAt).Suffix("ON CONFLICT (mmUserID, msTeamsUserID) DO UPDATE SET token = EXCLUDED.token, lastConnectAt = EXCLUDED.lastConnectAt, lastDisconnectAt = EXCLUDED.lastDisconnectAt").Exec(); err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) deleteUserInfo(db sq.BaseRunner, mmUserID string) error {
	if _, err := s.getQueryBuilder(db).Delete(usersTableName).Where(sq.Eq{"mmUserID": mmUserID}).Exec(); err != nil {
		return err
	}

	return nil
}

//db:withReplica
func (s *SQLStore) listChatSubscriptionsToCheck(db sq.BaseRunner) ([]storemodels.ChatSubscription, error) {
	expireTime := time.Now().Add(subscriptionRefreshTimeLimit).UnixMicro()
	query := s.getQueryBuilder(db).Select("subscriptionID, msTeamsUserID, secret, expiresOn, certificate").From(subscriptionsTableName).Where(sq.Eq{"type": subscriptionTypeUser}).Where(sq.Lt{"expiresOn": expireTime})
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

//db:withReplica
func (s *SQLStore) listChannelSubscriptions(db sq.BaseRunner) ([]*storemodels.ChannelSubscription, error) {
	query := s.getQueryBuilder(db).Select("subscriptionID, msTeamsChannelID, msTeamsTeamID, secret, expiresOn, certificate").From(subscriptionsTableName).Where(sq.Eq{"type": subscriptionTypeChannel})
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

//db:withReplica
func (s *SQLStore) listChannelSubscriptionsToRefresh(db sq.BaseRunner, certificate string) ([]*storemodels.ChannelSubscription, error) {
	expireTime := time.Now().Add(subscriptionRefreshTimeLimit).UnixMicro()
	query := s.getQueryBuilder(db).
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

//db:withReplica
func (s *SQLStore) listGlobalSubscriptions(db sq.BaseRunner) ([]*storemodels.GlobalSubscription, error) {
	query := s.getQueryBuilder(db).Select("subscriptionID, type, secret, expiresOn, certificate").From(subscriptionsTableName).Where(sq.Eq{"type": subscriptionTypeAllChats})
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

//db:withReplica
func (s *SQLStore) listGlobalSubscriptionsToRefresh(db sq.BaseRunner, certificate string) ([]*storemodels.GlobalSubscription, error) {
	expireTime := time.Now().Add(subscriptionRefreshTimeLimit).UnixMicro()
	query := s.getQueryBuilder(db).
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

//db:withTransaction
func (s *SQLStore) saveGlobalSubscription(db sq.BaseRunner, subscription storemodels.GlobalSubscription) error {
	if _, err := s.getQueryBuilder(db).Delete(subscriptionsTableName).Where(sq.Eq{"type": subscription.Type}).Exec(); err != nil {
		return err
	}

	if _, err := s.getQueryBuilder(db).Insert(subscriptionsTableName).Columns("subscriptionID, type, secret, expiresOn, certificate").Values(subscription.SubscriptionID, subscription.Type, subscription.Secret, subscription.ExpiresOn.UnixMicro(), subscription.Certificate).Exec(); err != nil {
		return err
	}
	return nil
}

//db:withTransaction
func (s *SQLStore) saveChatSubscription(db sq.BaseRunner, subscription storemodels.ChatSubscription) error {
	if _, err := s.getQueryBuilder(db).Delete(subscriptionsTableName).Where(sq.Eq{"msteamsUserID": subscription.UserID}).Exec(); err != nil {
		return err
	}

	if _, err := s.getQueryBuilder(db).Insert(subscriptionsTableName).Columns("subscriptionID, msTeamsUserID, type, secret, expiresOn, certificate").Values(subscription.SubscriptionID, subscription.UserID, subscriptionTypeUser, subscription.Secret, subscription.ExpiresOn.UnixMicro(), subscription.Certificate).Exec(); err != nil {
		return err
	}
	return nil
}

//db:withTransaction
func (s *SQLStore) saveChannelSubscription(db sq.BaseRunner, subscription storemodels.ChannelSubscription) error {
	if _, err := s.getQueryBuilder(db).Delete(subscriptionsTableName).Where(sq.Eq{"msTeamsTeamID": subscription.TeamID, "msTeamsChannelID": subscription.ChannelID}).Exec(); err != nil {
		return err
	}

	if _, err := s.getQueryBuilder(db).Insert(subscriptionsTableName).Columns("subscriptionID, msTeamsTeamID, msTeamsChannelID, type, secret, expiresOn, certificate").Values(subscription.SubscriptionID, subscription.TeamID, subscription.ChannelID, subscriptionTypeChannel, subscription.Secret, subscription.ExpiresOn.UnixMicro(), subscription.Certificate).Exec(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) updateSubscriptionExpiresOn(db sq.BaseRunner, subscriptionID string, expiresOn time.Time) error {
	query := s.getQueryBuilder(db).Update(subscriptionsTableName).Set("expiresOn", expiresOn.UnixMicro()).Where(sq.Eq{"subscriptionID": subscriptionID})
	_, err := query.Exec()
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) updateSubscriptionLastActivityAt(db sq.BaseRunner, subscriptionID string, lastActivityAt time.Time) error {
	query := s.getQueryBuilder(db).
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

//db:withReplica
func (s *SQLStore) getSubscriptionsLastActivityAt(db sq.BaseRunner) (map[string]time.Time, error) {
	query := s.getQueryBuilder(db).
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

func (s *SQLStore) deleteSubscription(db sq.BaseRunner, subscriptionID string) error {
	if _, err := s.getQueryBuilder(db).Delete(subscriptionsTableName).Where(sq.Eq{"subscriptionID": subscriptionID}).Exec(); err != nil {
		return err
	}
	return nil
}

//db:withReplica
func (s *SQLStore) getChannelSubscription(db sq.BaseRunner, subscriptionID string) (*storemodels.ChannelSubscription, error) {
	row := s.getQueryBuilder(db).Select("subscriptionID, msTeamsChannelID, msTeamsTeamID, secret, expiresOn, certificate").From(subscriptionsTableName).Where(sq.Eq{"subscriptionID": subscriptionID, "type": subscriptionTypeChannel}).Suffix("FOR UPDATE").QueryRow()
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

//db:withReplica
func (s *SQLStore) getChannelSubscriptionByTeamsChannelID(db sq.BaseRunner, teamsChannelID string) (*storemodels.ChannelSubscription, error) {
	row := s.getQueryBuilder(db).Select("subscriptionID").From(subscriptionsTableName).Where(sq.Eq{"msTeamsChannelID": teamsChannelID, "type": subscriptionTypeChannel}).QueryRow()
	var subscription storemodels.ChannelSubscription
	if scanErr := row.Scan(&subscription.SubscriptionID); scanErr != nil {
		return nil, scanErr
	}
	return &subscription, nil
}

//db:withReplica
func (s *SQLStore) getChatSubscription(db sq.BaseRunner, subscriptionID string) (*storemodels.ChatSubscription, error) {
	row := s.getQueryBuilder(db).Select("subscriptionID, msTeamsUserID, secret, expiresOn, certificate").From(subscriptionsTableName).Where(sq.Eq{"subscriptionID": subscriptionID, "type": subscriptionTypeUser}).QueryRow()
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

//db:withReplica
func (s *SQLStore) getGlobalSubscription(db sq.BaseRunner, subscriptionID string) (*storemodels.GlobalSubscription, error) {
	row := s.getQueryBuilder(db).Select("subscriptionID, type, secret, expiresOn, certificate").From(subscriptionsTableName).Where(sq.Eq{"subscriptionID": subscriptionID, "type": subscriptionTypeAllChats}).QueryRow()
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

//db:withReplica
func (s *SQLStore) getSubscriptionType(db sq.BaseRunner, subscriptionID string) (string, error) {
	row := s.getQueryBuilder(db).Select("type").From(subscriptionsTableName).Where(sq.Eq{"subscriptionID": subscriptionID}).QueryRow()
	var subscriptionType string
	if scanErr := row.Scan(&subscriptionType); scanErr != nil {
		return "", scanErr
	}
	return subscriptionType, nil
}

func (s *SQLStore) recoverPost(db sq.BaseRunner, postID string) error {
	query := s.getQueryBuilder(db).Update("Posts").Set("DeleteAt", 0).Where(sq.Eq{"Id": postID}, sq.NotEq{"DeleteAt": 0})
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

func (s *SQLStore) getQueryBuilder(db sq.BaseRunner) sq.StatementBuilderType {
	return sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(db)
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

//db:withReplica
func (s *SQLStore) getLinkedChannelsCount(db sq.BaseRunner) (linkedChannels int64, err error) {
	err = s.getQueryBuilder(db).
		Select("count(mmChannelID)").
		From(linksTableName).
		QueryRow().
		Scan(&linkedChannels)

	return linkedChannels, err
}

//db:withReplica
func (s *SQLStore) getConnectedUsersCount(db sq.BaseRunner) (connectedUsers int64, err error) {
	err = s.getQueryBuilder(db).
		Select("count(mmUserID)").
		From(usersTableName).
		Where(sq.And{
			sq.NotEq{"token": ""},
			sq.NotEq{"token": nil},
		}).
		QueryRow().
		Scan(&connectedUsers)

	return connectedUsers, err
}

//db:withReplica
func (s *SQLStore) getSyntheticUsersCount(db sq.BaseRunner, remoteID string) (syntheticUsers int64, err error) {
	err = s.getQueryBuilder(db).
		Select("count(id)").
		From("users").
		Where(sq.And{
			sq.Eq{"RemoteId": remoteID},
			sq.Or{
				sq.Eq{"DeleteAt": 0},
				sq.Eq{"DeleteAt": nil},
			},
		}).
		QueryRow().
		Scan(&syntheticUsers)

	return syntheticUsers, err
}

//db:withReplica
func (s *SQLStore) getUsersByPrimaryPlatformsCount(db sq.BaseRunner, preferenceCategory string) (msTeamsPrimary, mmPrimary int64, err error) {
	query := s.getQueryBuilder(db).
		Select("p.value", "count(*)").
		From("preferences p").
		LeftJoin(fmt.Sprintf("%s u ON p.userid = u.mmuserid", usersTableName)).
		Where(sq.And{
			sq.Eq{"p.category": preferenceCategory},
			sq.Eq{"p.name": storemodels.PreferenceNamePlatform},
			sq.And{sq.NotEq{"u.token": nil}, sq.NotEq{"u.token": ""}},
		}).
		GroupBy("p.value")
	rows, err := query.Query()
	if err != nil {
		return msTeamsPrimary, mmPrimary, err
	}
	defer rows.Close()

	for rows.Next() {
		var platform string
		var count int64
		if err := rows.Scan(&platform, &count); err != nil {
			return msTeamsPrimary, mmPrimary, err
		}

		switch platform {
		case storemodels.PreferenceValuePlatformMM:
			mmPrimary = count
		case storemodels.PreferenceValuePlatformMSTeams:
			msTeamsPrimary = count
		}
	}

	return msTeamsPrimary, mmPrimary, nil
}

//db:withReplica
func (s *SQLStore) getActiveUsersSendingCount(db sq.BaseRunner, dur time.Duration) (activeUsersSending int64, err error) {
	now := time.Now()

	err = s.getQueryBuilder(db).
		Select("count(*)").
		From(usersTableName).
		Where(sq.GtOrEq{"LastChatSentAt": now.Add(-dur).UnixMicro()}).
		Where(sq.LtOrEq{"LastChatSentAt": now.UnixMicro()}).
		QueryRow().
		Scan(&activeUsersSending)

	return activeUsersSending, err
}

//db:withReplica
func (s *SQLStore) getActiveUsersReceivingCount(db sq.BaseRunner, dur time.Duration) (activeUsersReceiving int64, err error) {
	now := time.Now()

	err = s.getQueryBuilder(db).
		Select("count(*)").
		From(usersTableName).
		Where(sq.GtOrEq{"LastChatReceivedAt": now.Add(-dur).UnixMicro()}).
		Where(sq.LtOrEq{"LastChatReceivedAt": now.UnixMicro()}).
		QueryRow().
		Scan(&activeUsersReceiving)

	return activeUsersReceiving, err
}

//db:withReplica
func (s *SQLStore) getConnectedUsers(db sq.BaseRunner, page, perPage int) ([]*storemodels.ConnectedUser, error) {
	query := s.getQueryBuilder(db).Select("mmuserid, msteamsuserid, Users.FirstName, Users.LastName, Users.Email").From(usersTableName).LeftJoin("Users ON Users.Id = msteamssync_users.mmuserid").Where(sq.NotEq{"token": ""}).OrderBy("Users.FirstName").Offset(uint64(page * perPage)).Limit(uint64(perPage))
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

//db:withReplica
func (s *SQLStore) getHasConnectedCount(db sq.BaseRunner) (int, error) {
	query := s.getQueryBuilder(db).
		Select("count(*)").
		From(usersTableName).
		Where(sq.And{sq.NotEq{"lastConnectAt": 0}})
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

func (s *SQLStore) storeUserInWhitelist(db sq.BaseRunner, userID string) error {
	query := s.getQueryBuilder(db).Insert(whitelistTableName).Columns("mmUserID").Values(userID)
	if _, err := query.Exec(); err != nil {
		if isDuplicate(err) {
			s.api.LogDebug("UserID already present in whitelist", "UserID", userID)
			return nil
		}

		return err
	}

	return nil
}

func (s *SQLStore) storeUsersInWhitelist(db sq.BaseRunner, userIDs []string) error {
	query := s.getQueryBuilder(db).
		Insert(whitelistTableName).
		Columns("mmUserID")

	for _, userID := range userIDs {
		query = query.Values(userID)
	}

	if _, err := query.Exec(); err != nil {
		// TODO handle duplicates
		return err
	}

	return nil
}

//db:withTransaction
func (s *SQLStore) setWhitelist(db sq.BaseRunner, userIDs []string, batchSize int) error {
	if err := s.deleteWhitelist(db); err != nil {
		s.api.LogDebug("Error deleting whitelist")
		return err
	}

	var currentBatch []string

	for i, id := range userIDs {
		currentBatch = append(currentBatch, id)
		if len(currentBatch) >= batchSize || i == len(userIDs)-1 {
			// batch threshold met, or end of list
			if err := s.storeUsersInWhitelist(db, currentBatch); err != nil {
				s.api.LogDebug("Error adding batched users to whitelist", "error", err.Error(), "userIds", currentBatch)
				return err
			}
			currentBatch = nil
		}
	}

	return nil
}

//db:withReplica
func (s *SQLStore) isUserWhitelisted(db sq.BaseRunner, userID string) (bool, error) {
	query := s.getQueryBuilder(db).Select("mmUserID").From(whitelistTableName).Where(sq.Eq{"mmUserID": userID})
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

func (s *SQLStore) deleteUserFromWhitelist(db sq.BaseRunner, mmUserID string) error {
	if _, err := s.getQueryBuilder(db).Delete(whitelistTableName).Where(sq.Eq{"mmUserID": mmUserID}).Exec(); err != nil {
		return err
	}

	return nil
}

//db:withReplica
func (s *SQLStore) getWhitelistCount(db sq.BaseRunner) (int, error) {
	query := s.getQueryBuilder(db).Select("count(*)").From(whitelistTableName)
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

//db:withReplica
func (s *SQLStore) getWhitelistEmails(db sq.BaseRunner, page, perPage int) ([]string, error) {
	query := s.getQueryBuilder(db).
		Select("Users.Email").
		From(whitelistTableName).
		LeftJoin("Users ON Users.Id = msteamssync_whitelist.mmuserid").
		Offset(uint64(page * perPage)).
		Limit(uint64(perPage))
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			s.api.LogDebug("Unable to scan the result", "Error", err.Error())
			continue
		}

		result = append(result, email)
	}

	return result, nil
}

func (s *SQLStore) deleteWhitelist(db sq.BaseRunner) error {
	if _, err := s.getQueryBuilder(db).Delete(whitelistTableName).Exec(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) storeInvitedUser(db sq.BaseRunner, invitedUser *storemodels.InvitedUser) error {
	pendingSince := invitedUser.InvitePendingSince.UnixMicro()
	lastSentAt := invitedUser.InviteLastSentAt.UnixMicro()

	query := s.getQueryBuilder(db).
		Insert(invitedUsersTableName).
		Columns("mmUserID", "invitePendingSince", "inviteLastSentAt").
		Values(invitedUser.ID, pendingSince, lastSentAt).
		SuffixExpr(sq.Expr("ON CONFLICT (mmUserID) DO UPDATE SET invitePendingSince = ?, inviteLastSentAt = ?", pendingSince, lastSentAt))

	if _, err := query.Exec(); err != nil {
		return err
	}

	if err := s.DeleteUserFromWhitelist(invitedUser.ID); err != nil {
		return err
	}

	return nil
}

//db:withReplica
func (s *SQLStore) getInvitedUser(db sq.BaseRunner, mmUserID string) (*storemodels.InvitedUser, error) {
	query := s.getQueryBuilder(db).
		Select("mmUserID", "invitePendingSince", "inviteLastSentAt").
		From(invitedUsersTableName).
		Where(sq.Eq{"mmUserID": mmUserID})

	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var result = &storemodels.InvitedUser{}
		var pendingSince int64
		var lastSentAt int64

		if scanErr := rows.Scan(&result.ID, &pendingSince, &lastSentAt); scanErr != nil {
			return nil, scanErr
		}

		if pendingSince != 0 {
			result.InvitePendingSince = time.UnixMicro(pendingSince)
		}

		if lastSentAt != 0 {
			result.InvitePendingSince = time.UnixMicro(lastSentAt)
		}

		return result, nil
	}

	return nil, nil
}

func (s *SQLStore) deleteUserInvite(db sq.BaseRunner, mmUserID string) error {
	if _, err := s.getQueryBuilder(db).Delete(invitedUsersTableName).Where(sq.Eq{"mmUserID": mmUserID}).Exec(); err != nil {
		return err
	}

	return nil
}

//db:withReplica
func (s *SQLStore) getInvitedCount(db sq.BaseRunner) (int, error) {
	query := s.getQueryBuilder(db).Select("count(*)").From(invitedUsersTableName)
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

func (s *SQLStore) setUserLastChatSentAt(db sq.BaseRunner, mmUserID string, sentAt int64) error {
	query := s.getQueryBuilder(db).
		Update(usersTableName).
		Set("LastChatSentAt", sentAt).
		Where(sq.And{
			sq.Eq{"mmUserID": mmUserID},
			sq.Lt{"LastChatSentAt": sentAt}, // Make sure we store the latest value
		})
	if _, err := query.Exec(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) setUserLastChatReceivedAt(db sq.BaseRunner, mmUserID string, receivedAt int64) error {
	return s.setUsersLastChatReceivedAt(db, []string{mmUserID}, receivedAt)
}

func (s *SQLStore) setUsersLastChatReceivedAt(db sq.BaseRunner, mmUsersID []string, receivedAt int64) error {
	query := s.getQueryBuilder(db).
		Update(usersTableName).
		Set("LastChatReceivedAt", receivedAt).
		Where(sq.And{
			sq.Eq{"mmUserID": mmUsersID},
			sq.Lt{"LastChatReceivedAt": receivedAt}, // Make sure we store the latest value
		})
	if _, err := query.Exec(); err != nil {
		return err
	}

	return nil
}
