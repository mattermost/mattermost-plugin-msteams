package sqlstore

import (
	"context"
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
		"users.username",
		"users.remoteid",
	).
		From("msteamssync_users").
		Where(sq.Expr("msteamsuserid IN ( SELECT msteamsuserid FROM msteamssync_users GROUP BY msteamsuserid HAVING COUNT(*) > 1)")).
		Join("users ON msteamssync_users.mmuserid = users.id").
		OrderBy("users.createat ASC").
		Query()
	if err != nil {
		return err
	}

	bestCandidate := map[string]string{}
	bestCandidateScore := map[string]byte{}
	var mmuserid, msteamsuserid, username, remoteid string
	var nRemoteid sql.NullString
	for rows.Next() {
		err = rows.Scan(&mmuserid, &msteamsuserid, &username, &nRemoteid)
		if err != nil {
			return err
		}

		remoteid = ""
		if nRemoteid.Valid {
			remoteid = nRemoteid.String
		}

		currentUserScore := DedupScoreDefault
		if remoteid == "" {
			currentUserScore = DedupScoreNotSynthetic
		}

		_, ok := bestCandidate[msteamsuserid]
		if !ok {
			bestCandidate[msteamsuserid] = mmuserid
			bestCandidateScore[msteamsuserid] = currentUserScore
			continue
		}

		if ok && currentUserScore > bestCandidateScore[msteamsuserid] {
			bestCandidate[msteamsuserid] = mmuserid
			bestCandidateScore[msteamsuserid] = currentUserScore
			continue
		}
	}

	// for each msteamsusers, remove all but the best candidate
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}

	defer func() { _ = tx.Rollback() }()
	for msteamsuserid, mmuserid := range bestCandidate {
		s.api.LogInfo("Deleting duplicates for msteamsuserid: " + msteamsuserid + ", keeping mmuserid: " + mmuserid)
		_, err := s.getQueryBuilderWithRunner(tx).Delete("msteamssync_users").
			Where(sq.And{
				sq.Eq{"msteamsuserid": msteamsuserid},
				sq.NotEq{"mmuserid": mmuserid},
			}).
			Exec()
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
