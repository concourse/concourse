package migrations

import "github.com/BurntSushi/migration"

func DropCompletedFromBuildPreparation(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	ALTER TABLE build_preparation
	DROP COLUMN completed
	`)
	return err
}
