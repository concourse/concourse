package migrations

import "github.com/concourse/atc/db/migration"

func AddInputsSatisfiedToBuildPreparation(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	ALTER TABLE build_preparation
	ADD COLUMN inputs_satisfied text NOT NULL DEFAULT 'unknown'
	`)
	return err
}
