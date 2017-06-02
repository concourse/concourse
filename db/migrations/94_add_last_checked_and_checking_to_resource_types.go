package migrations

import "github.com/concourse/atc/db/migration"

func AddLastCheckedAndCheckingToResourceTypes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resource_types
		ADD COLUMN last_checked timestamp NOT NULL DEFAULT 'epoch',
		ADD COLUMN checking bool NOT NULL DEFAULT false
	`)
	return err
}
