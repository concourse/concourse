package migrations

import "github.com/concourse/atc/db/migration"

func RemoveSizeInBytesFromVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes DROP size_in_bytes;
	`)

	if err != nil {
		return err
	}

	return nil
}
