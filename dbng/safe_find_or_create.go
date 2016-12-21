package dbng

import "errors"

var ErrSafeRetryFindOrCreate = errors.New("failed-to-run-safe-find-or-create-retrying")

func safeFindOrCreate(conn Conn, findOrCreateFunc func(tx Tx) error) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}

	// didn't forget defer tx.Rollback() - just don't need it.

	err = findOrCreateFunc(tx)
	if err != nil {
		tx.Rollback()

		if err == ErrSafeRetryFindOrCreate {
			return safeFindOrCreate(conn, findOrCreateFunc)
		}

		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
