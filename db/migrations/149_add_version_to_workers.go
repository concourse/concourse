package migrations

import "github.com/concourse/atc/db/migration"

func AddVersionToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN version text;
`)
	return err
}
