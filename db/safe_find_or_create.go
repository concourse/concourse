package db

import (
	"errors"
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
			_ = tx.Rollback()

			if err == ErrSafeRetryFindOrCreate {
				continue
			}

			return err
		}

		return tx.Commit()
	}
}
