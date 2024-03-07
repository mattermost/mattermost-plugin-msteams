package sqlstore

func (s *SQLStore) runMigrationRemoteID(remoteID string) error {
	_, err := s.db.Exec("UPDATE Users SET RemoteID = $1 WHERE RemoteID IS NOT NULL AND RemoteID != '' AND RemoteID NOT IN (SELECT remoteid FROM remoteclusters) AND username like 'msteams%'", remoteID)
	return err
}
