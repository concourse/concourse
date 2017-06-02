package migrations

import "github.com/concourse/atc/db/migration"

func RenameConfigToPipelines(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	ALTER TABLE config
    RENAME TO pipelines
	`)
	return err
}
