package sqlstore

import (
	"fmt"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
)

const (
	legacyRemoteIDMigrationKey                             = "RemoteIDMigrationComplete"
	legacySetEmailVerifiedToTrueForRemoteUsersMigrationKey = "SetEmailVerifiedToTrueForRemoteUsersMigrationComplete"
	legacyMSTeamUserIDDedupMigrationKey                    = "MSTeamUserIDDedupMigrationComplete"
	WhitelistedUsersMigrationKey                           = "WhitelistedUsersMigrationComplete"
)

func (s *SQLStore) runWhitelistedUsersMigration() error {
	setting, err := s.getSystemSetting(s.db, WhitelistedUsersMigrationKey)
	if err != nil {
		return fmt.Errorf("cannot get Whitelisted Users migration state: %w", err)
	}

	if hasAlreadyRun, _ := strconv.ParseBool(setting); hasAlreadyRun {
		return nil
	}

	oldWhitelistToProcess, err := tableExist(s, whitelistedUsersLegacyTableName)
	if err != nil {
		return err
	}

	if !oldWhitelistToProcess {
		// migration already done, no rows to process
		if sErr := s.setSystemSetting(s.db, WhitelistedUsersMigrationKey, strconv.FormatBool(true)); sErr != nil {
			return fmt.Errorf("cannot mark Whitelisted Users migration as completed: %w", sErr)
		}

		return nil
	}

	s.api.LogDebug("Running Whitelisted Users migration")

	now := time.Now()

	// all presently-whitelisted users should already in the users table,
	// as being added to the old whitelist only happened after successful connection.

	// has-connected users (presently and previously)
	_, err = s.getQueryBuilder(s.db).
		Update(usersTableName).
		Set("lastConnectAt", now.UnixMicro()).
		Where(sq.Or{
			sq.And{sq.NotEq{"token": ""}, sq.NotEq{"token": nil}},
			sq.Expr("mmUserID IN (SELECT mmUserID FROM " + whitelistedUsersLegacyTableName + ")"),
		}).
		Exec()
	if err != nil {
		return err
	}

	// only previously-connected
	_, err = s.getQueryBuilder(s.db).
		Update(usersTableName).
		Set("lastDisconnectAt", now.UnixMicro()).
		Where(sq.And{
			sq.Or{sq.Eq{"token": ""}, sq.Eq{"token": nil}},
			sq.Expr("mmUserID IN (SELECT mmUserID FROM " + whitelistedUsersLegacyTableName + ")"),
		}).
		Exec()
	if err != nil {
		return err
	}

	if err := s.setSystemSetting(s.db, WhitelistedUsersMigrationKey, strconv.FormatBool(true)); err != nil {
		return fmt.Errorf("cannot mark Whitelisted Users migration as completed: %w", err)
	}

	s.api.LogDebug("Whitelisted Users migration run successfully", "elapsed", time.Since(now))

	return nil
}
