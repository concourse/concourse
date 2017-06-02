package migrations

import "github.com/concourse/atc/db/migration"

func AlterExpiresToIncludeTimezoneInWorkers(tx migration.LimitedTx) error {

	_, err := tx.Exec(`
		ALTER TABLE workers
		ALTER COLUMN expires type timestamp with time zone;
	`)
	if err != nil {
		return err
	}

	return nil
}
