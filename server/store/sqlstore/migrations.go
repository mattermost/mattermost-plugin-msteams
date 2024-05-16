package sqlstore

import (
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
)

func (s *SQLStore) runMigrationRemoteID(remoteID string) error {
	_, err := s.getQueryBuilder(s.db).Update("Users").Set("RemoteID", remoteID).Where(sq.And{
		sq.NotEq{"RemoteID": nil},
		sq.NotEq{"RemoteID": ""},
		sq.Expr("RemoteID NOT IN (SELECT remoteid FROM remoteclusters)"),
		sq.Like{"Username": "msteams_%"},
	}).Exec()
	return err
}

func (s *SQLStore) runSetEmailVerifiedToTrueForRemoteUsers(remoteID string) error {
	_, err := s.getQueryBuilder(s.db).
		Update("Users").
		Set("EmailVerified", true).
		Where(sq.And{
			sq.Eq{"RemoteID": remoteID},
			sq.Eq{"EmailVerified": false},
		}).Exec()

	return err
}

const (
	DedupScoreDefault      byte = 0
	DedupScoreNotSynthetic byte = 1
)

func (s *SQLStore) runMSTeamUserIDDedup() error {
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

	return err
}

func (s *SQLStore) ensureMigrationWhitelistedUsers() error {
	oldWhitelistToProcess, err := s.tableExist(whitelistedUsersLegacyTableName)
	if err != nil {
		return err
	}

	if !oldWhitelistToProcess {
		// migration already done, no rows to process
		return nil
	}

	s.api.LogInfo("Migrating old whitelist rows")

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

	err = s.deleteTable(whitelistedUsersLegacyTableName)

	if err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) createTable(tableName, columnList string) error {
	if _, err := s.db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, columnList)); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) deleteTable(tableName string) error {
	if _, err := s.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)); err != nil {
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

func (s *SQLStore) createUniqueIndex(tableName, indexName, columnList string) error {
	if _, err := s.db.Exec(fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (%s)", indexName, tableName, columnList)); err != nil {
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

func (s *SQLStore) indexExist(tableName, indexName string) (bool, error) {
	rows, err := s.db.Query(fmt.Sprintf("SELECT 1 FROM pg_indexes WHERE tablename = '%s' AND indexname = '%s'", tableName, indexName))
	if err != nil {
		return false, err
	}

	defer rows.Close()
	return rows.Next(), nil
}

func (s *SQLStore) tableExist(tableName string) (bool, error) {
	rows, err := s.db.Query(fmt.Sprintf("SELECT 1 FROM pg_tables WHERE schemaname = current_schema() AND tablename = '%s'", tableName))
	if err != nil {
		return false, err
	}

	defer rows.Close()
	return rows.Next(), nil
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

func (s *SQLStore) createMSTeamsUserIDUniqueIndex() error {
	return s.createUniqueIndex(usersTableName, "idx_msteamssync_users_msteamsuserid_unq", "msteamsuserid")
}
