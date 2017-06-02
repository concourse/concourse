package migrations

import "github.com/concourse/atc/db/migration"

func AddStartTimeToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN start_time integer;
`)
	return err
}
