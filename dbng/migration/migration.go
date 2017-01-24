package migration

import (
	"database/sql"
	"fmt"
)

type LimitedTx interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Stmt(stmt *sql.Stmt) *sql.Stmt
}
type Migrator func(LimitedTx) error

func Open(driver, dsn string, migrations []Migrator) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS migration_version (version INTEGER)")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
	INSERT INTO migration_version
	    (version)
	SELECT 0
	WHERE
	    NOT EXISTS (
		SELECT * FROM migration_version
	    );`)
	if err != nil {
		return nil, err
	}

	var dbVersion int
	r := db.QueryRow("SELECT version FROM migration_version")
	if err := r.Scan(&dbVersion); err != nil {
		return nil, err
	}

	libVersion := len(migrations)

	if dbVersion > libVersion {
		return nil, fmt.Errorf("Database version (%d) is greater than library version (%d).",
			dbVersion, libVersion)
	}
	if dbVersion == libVersion {
		return db, nil
	}

	var (
		tx *sql.Tx
	)

	for i, m := range migrations {
		version := i + 1
		if version <= dbVersion {
			continue
		}

		tx, err = db.Begin()
		if err != nil {
			return nil, err
		}

		err = m(tx)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		_, err = tx.Exec("UPDATE migration_version SET version = $1", version)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		err = tx.Commit()
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	return db, nil
}
