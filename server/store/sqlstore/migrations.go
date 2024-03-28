package sqlstore

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

func (s *SQLStore) runMigrationRemoteID(remoteID string) error {
	_, err := s.getQueryBuilder().Update("Users").Set("RemoteID", remoteID).Where(sq.And{
		sq.NotEq{"RemoteID": nil},
		sq.NotEq{"RemoteID": ""},
		sq.Expr("RemoteID NOT IN (SELECT remoteid FROM remoteclusters)"),
		sq.Like{"Username": "msteams_%"},
	}).Exec()
	return err
}

const (
	DedupScoreDefault      byte = 0
	DedupScoreNotSynthetic byte = 1
)

func (s *SQLStore) runMSTeamUserIDDedup() error {
	rows, err := s.getQueryBuilder().Select(
		"msteamssync_users.mmuserid",
		"msteamssync_users.msteamsuserid",
		"users.remoteid",
	).
		From("msteamssync_users").
		Where(sq.Expr("msteamsuserid IN ( SELECT msteamsuserid FROM msteamssync_users GROUP BY msteamsuserid HAVING COUNT(*) > 1)")).
		LeftJoin("users ON msteamssync_users.mmuserid = users.id").
		OrderBy("users.createat ASC").
		Query()
	if err != nil {
		return err
	}

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

	orCond := sq.Or{}
	for teamsUserID, mmUserID := range bestCandidate {
		orCond = append(orCond, sq.And{
			sq.Eq{"msteamsuserid": teamsUserID},
			sq.NotEq{"mmuserid": mmUserID},
		})
	}

	s.api.LogInfo("Deleting duplicates")
	_, err = s.getQueryBuilder().Delete("msteamssync_users").
		Where(orCond).
		Exec()

	return err
}

func (s *SQLStore) createMSTeamsUserIdUniqueIndex() error {
	return s.createUniqueIndex(usersTableName, "idx_msteamssync_users_msteamsuserid_unq", "msteamsuserid")
}
