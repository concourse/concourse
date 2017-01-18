package migrations

import "github.com/concourse/atc/dbng/migration"

func DropCompletedFromBuildPreparation(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	ALTER TABLE build_preparation
	DROP COLUMN completed
	`)
	return err
}
