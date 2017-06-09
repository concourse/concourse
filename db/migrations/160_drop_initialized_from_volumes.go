package migrations

import "github.com/concourse/atc/db/migration"

func DropInitializedFromVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		DROP COLUMN initialized
`)
	if err != nil {
		return err
	}

	return nil
}
