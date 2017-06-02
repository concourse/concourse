package migrations

import "github.com/concourse/atc/db/migration"

func AddBestIfUsedByToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN best_if_used_by timestamp;
`)
	return err
}
