package migrations

import "github.com/concourse/atc/db/migration"

func MoveSourceAndMetadataToVersionedResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE versioned_resources
		ADD COLUMN source text,
		ADD COLUMN metadata text
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE build_inputs
		DROP COLUMN source,
		DROP COLUMN metadata
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE build_outputs
		DROP COLUMN source,
		DROP COLUMN metadata
	`)
	if err != nil {
		return err
	}

	return nil
}
