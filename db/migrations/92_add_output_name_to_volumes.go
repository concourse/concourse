package migrations

import "github.com/concourse/atc/db/migration"

func AddOutputNameToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ADD COLUMN output_name text DEFAULT null;
	`)
	if err != nil {
		return err
	}

	return nil
}
