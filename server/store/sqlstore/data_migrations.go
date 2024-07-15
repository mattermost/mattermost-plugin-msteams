package sqlstore

import (
	"database/sql"
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
	ArchiveSyntheticUsersMigrationKey                      = "ArchiveSyntheticUsersMigrationComplete"
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

func (s *SQLStore) runArchiveSyntheticUsersMigration() error {
	setting, err := s.getSystemSetting(s.db, ArchiveSyntheticUsersMigrationKey)
	if err != nil {
		return fmt.Errorf("cannot get Remote ID migration state: %w", err)
	}

	if hasAlreadyRun, _ := strconv.ParseBool(setting); hasAlreadyRun {
		return nil
	}

	s.api.LogDebug("Running Archive Synthetic Users migration")

	rows, err := s.getQueryBuilder(s.db).Select("id").From("Users").Where(sq.And{
		sq.Like{"Username": "msteams_%"},
		sq.Eq{"DeleteAt": 0},
	}).Query()
	if err != nil {
		return fmt.Errorf("failed to get unarchived synthetic users: %w", err)
	}
	defer rows.Close()

	numArchived := 0
	for rows.Next() {
		var userID string
		if err = rows.Scan(&userID); err != nil {
			return fmt.Errorf("failed to scan user id result: %w", err)
		}

		// Ensure the synthetic user is no longer mapped to a Teams user.
		if err = s.deleteUserInfo(s.db, userID); err != sql.ErrNoRows && err != nil {
			return fmt.Errorf("failed to unlink user id %s: %w", userID, err)
		} else if err == nil {
			s.api.LogInfo("Deleted synthetic user info", "user_id", userID)
		}

		// Archive the user via the API vs. direct SQL queries.
		appErr := s.api.DeleteUser(userID)
		if appErr != nil {
			return fmt.Errorf("failed to archive synthetic user %s: %w", userID, err)
		}
		s.api.LogInfo("Archived synthetic user", "user_id", userID)
		numArchived++
	}

	if err := s.setSystemSetting(s.db, ArchiveSyntheticUsersMigrationKey, strconv.FormatBool(true)); err != nil {
		return fmt.Errorf("cannot mark Archive Synthetic Users migration as completed: %w", err)
	}

	s.api.LogInfo("Archive Synthetic Users migration run successfully", "num_archived", numArchived)

	return nil
}
