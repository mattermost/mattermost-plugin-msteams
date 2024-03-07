package sqlstore

import (
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
