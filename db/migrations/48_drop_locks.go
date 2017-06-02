package migrations

import "github.com/concourse/atc/db/migration"

func DropLocks(tx migration.LimitedTx) error {
	_, err := tx.Exec("DROP TABLE locks")
	if err != nil {
		return err
	}

	return nil
}
