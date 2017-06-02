package migrations

import "github.com/concourse/atc/db/migration"

func AddAttemptsToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN attempts text NULL;
	`)
	return err
}
