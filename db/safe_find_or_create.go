package db

import (
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

var ErrSafeRetryFindOrCreate = errors.New("failed-to-run-safe-find-or-create-retrying")
var ErrSafeRetryCreateOrUpdate = errors.New("failed-to-run-safe-create-or-update-retrying")

func safeFindOrCreate(conn Conn, findOrCreateFunc func(tx Tx) error) error {
	for {
		tx, err := conn.Begin()
		if err != nil {
			return err
		}

		// didn't forget defer tx.Rollback() - just don't need it.

		err = findOrCreateFunc(tx)
		if err != nil {
			tx.Rollback()

			if err == ErrSafeRetryFindOrCreate {
				continue
			}

			return err
		}

		err = tx.Commit()
		if err != nil {
			return err
		}

		return nil
	}
}

func safeCreateOrUpdate(conn Conn, createFunc func(tx Tx) (sql.Result, error), updateFunc func(tx Tx) (sql.Result, error)) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	rows, err := updateFunc(tx)
	if err != nil {
		return err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		_, err := createFunc(tx)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
				tx.Rollback()
				return safeCreateOrUpdate(conn, createFunc, updateFunc)
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
