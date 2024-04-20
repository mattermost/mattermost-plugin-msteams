package sqlstore

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
)

const (
	RemoteIDMigrationKey                             = "RemoteIDMigrationComplete"
	SetEmailVerifiedToTrueForRemoteUsersMigrationKey = "SetEmailVerifiedToTrueForRemoteUsersMigrationComplete"
	MSTeamUserIDDedupMigrationKey                    = "MSTeamUserIDDedupMigrationComplete"
	WhitelistedUsersMigrationKey                     = "WhitelistedUsersMigrationComplete"

	DedupScoreDefault      byte = 0
	DedupScoreNotSynthetic byte = 1
)

func (s *SQLStore) runMigrationRemoteID(remoteID string) error {
	setting, err := s.getSystemSetting(RemoteIDMigrationKey)
	if err != nil {
		return fmt.Errorf("cannot get Remote ID migration state: %w", err)
	}

	if hasAlreadyRun, _ := strconv.ParseBool(setting); hasAlreadyRun {
		return nil
	}

	s.api.LogDebug("Running Remote ID migration")
	start := time.Now()

	_, qErr := s.getQueryBuilder(s.db).Update("Users").Set("RemoteID", remoteID).Where(sq.And{
		sq.NotEq{"RemoteID": nil},
		sq.NotEq{"RemoteID": ""},
		sq.Expr("RemoteID NOT IN (SELECT remoteid FROM remoteclusters)"),
		sq.Like{"Username": "msteams_%"},
	}).Exec()

	if qErr != nil {
		return qErr
	}

	if err := s.setSystemSetting(RemoteIDMigrationKey, strconv.FormatBool(true)); err != nil {
		return fmt.Errorf("cannot mark Remote ID migration as completed: %w", err)
	}

	s.api.LogDebug("Remote ID migration run successfully", "elapsed", time.Since(start))

	return nil
}

func (s *SQLStore) runSetEmailVerifiedToTrueForRemoteUsers(remoteID string) error {
	setting, err := s.getSystemSetting(SetEmailVerifiedToTrueForRemoteUsersMigrationKey)
	if err != nil {
		return fmt.Errorf("cannot get Set Email Verified to True for Remote Users migration state: %w", err)
	}

	if hasAlreadyRun, _ := strconv.ParseBool(setting); hasAlreadyRun {
		return nil
	}

	s.api.LogDebug("Running Set Email Verified to True for Remote Users migration")
	start := time.Now()

	_, qErr := s.getQueryBuilder(s.db).
		Update("Users").
		Set("EmailVerified", true).
		Where(sq.And{
			sq.Eq{"RemoteID": remoteID},
			sq.Eq{"EmailVerified": false},
		}).Exec()

	if qErr != nil {
		return qErr
	}

	if err := s.setSystemSetting(SetEmailVerifiedToTrueForRemoteUsersMigrationKey, strconv.FormatBool(true)); err != nil {
		return fmt.Errorf("cannot mark Set Email Verified to True for Remote Users migration as completed: %w", err)
	}

	s.api.LogDebug("Set Email Verified to True for Remote Users migration run successfully", "elapsed", time.Since(start))

	return nil
}

func (s *SQLStore) runMSTeamUserIDDedup() error {
	setting, err := s.getSystemSetting(MSTeamUserIDDedupMigrationKey)
	if err != nil {
		return fmt.Errorf("cannot get MSTeam User ID Dedup migration state: %w", err)
	}

	if hasAlreadyRun, _ := strconv.ParseBool(setting); hasAlreadyRun {
		return nil
	}

	s.api.LogDebug("Running MSTeam User ID Dedup migration")

	// get all users with duplicate msteamsuserid
	rows, err := s.getQueryBuilder(s.replica).Select(
		"mmuserid",
		"msteamsuserid",
		"remoteid",
	).
		From(usersTableName).
		Where(sq.Expr("msteamsuserid IN ( SELECT msteamsuserid FROM " + usersTableName + " GROUP BY msteamsuserid HAVING COUNT(*) > 1)")).
		LeftJoin("users ON " + usersTableName + ".mmuserid = users.id").
		OrderBy("users.createat ASC").
		Query()
	if err != nil {
		return err
	}

	// find the best candidate to keep based on:
	// 1. real user > synthetic user
	// 2. keep the oldest user
	bestCandidate := map[string]string{}
	bestCandidateScore := map[string]byte{}
	for rows.Next() {
		var mmUserID, teamsUserID, remoteID string
		var nRemoteID sql.NullString

		err = rows.Scan(&mmUserID, &teamsUserID, &nRemoteID)
		if err != nil {
			return err
		}

		remoteID = ""
		if nRemoteID.Valid {
			remoteID = nRemoteID.String
		}

		currentUserScore := DedupScoreDefault
		if remoteID == "" {
			currentUserScore = DedupScoreNotSynthetic
		}

		_, ok := bestCandidate[teamsUserID]
		if !ok {
			bestCandidate[teamsUserID] = mmUserID
			bestCandidateScore[teamsUserID] = currentUserScore
			continue
		}

		if ok && currentUserScore > bestCandidateScore[teamsUserID] {
			bestCandidate[teamsUserID] = mmUserID
			bestCandidateScore[teamsUserID] = currentUserScore
			continue
		}
	}

	if len(bestCandidate) == 0 {
		return nil
	}

	// for each msteams id, we're deleting all the duplicates except the best candidate
	orCond := sq.Or{}
	for teamsUserID, mmUserID := range bestCandidate {
		orCond = append(orCond, sq.And{
			sq.Eq{"msteamsuserid": teamsUserID},
			sq.NotEq{"mmuserid": mmUserID},
		})
	}

	s.api.LogInfo("Deleting duplicates")
	_, err = s.getQueryBuilder(s.db).Delete(usersTableName).
		Where(orCond).
		Exec()
	if err != nil {
		return err
	}

	if err := s.setSystemSetting(MSTeamUserIDDedupMigrationKey, strconv.FormatBool(true)); err != nil {
		return fmt.Errorf("cannot mark MSTeam User ID Dedup migration as completed: %w", err)
	}

	s.api.LogDebug("MSTeam User ID Dedup migration run successfully")

	return nil
}

func (s *SQLStore) runWhitelistedUsersMigration() error {
	setting, err := s.getSystemSetting(WhitelistedUsersMigrationKey)
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
		if sErr := s.setSystemSetting(WhitelistedUsersMigrationKey, strconv.FormatBool(true)); sErr != nil {
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

	if err := s.setSystemSetting(WhitelistedUsersMigrationKey, strconv.FormatBool(true)); err != nil {
		return fmt.Errorf("cannot mark Whitelisted Users migration as completed: %w", err)
	}

	s.api.LogDebug("Whitelisted Users migration run successfully", "elapsed", time.Since(now))

	return nil
}
