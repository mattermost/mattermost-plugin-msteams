package sqlstore

import (
	"fmt"
)

func createTable(store *SQLStore, tableName, columnList string) error {
	if _, err := store.db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, columnList)); err != nil {
		return err
	}

	return nil
}

func tableExist(store *SQLStore, tableName string) (bool, error) {
	rows, err := store.db.Query(fmt.Sprintf("SELECT 1 FROM pg_tables WHERE schemaname = current_schema() AND tablename = '%s'", tableName))
	if err != nil {
		return false, err
	}

	defer rows.Close()
	return rows.Next(), nil
}
