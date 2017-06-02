package migrations

import "github.com/concourse/atc/db/migration"

func AddOriginalVolumeHandleToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ADD COLUMN original_volume_handle text DEFAULT null;
	`)
	if err != nil {
		return err
	}

	return nil
}
