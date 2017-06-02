package migrations

import "github.com/concourse/atc/db/migration"

func NonNullableVersionInfo(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		UPDATE versioned_resources
		SET type = 'unknown'
		WHERE type IS NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE versioned_resources
		SET source = '{}'
		WHERE source IS NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE versioned_resources
		SET metadata = '[]'
		WHERE metadata IS NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources
		ALTER COLUMN type SET NOT NULL,
		ALTER COLUMN source SET NOT NULL,
		ALTER COLUMN metadata SET NOT NULL
	`)
	if err != nil {
		return err
	}

	return nil
}
