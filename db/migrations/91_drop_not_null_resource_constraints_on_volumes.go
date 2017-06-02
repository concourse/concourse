package migrations

import "github.com/concourse/atc/db/migration"

func DropNotNullResourceConstraintsOnVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ALTER COLUMN resource_version DROP NOT NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes ALTER COLUMN resource_hash DROP NOT NULL
	`)
	if err != nil {
		return err
	}

	return nil
}
