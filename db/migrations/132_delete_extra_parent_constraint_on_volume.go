package migrations

import "github.com/concourse/atc/db/migration"

func DeleteExtraParentConstrainOnVolume(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		DROP CONSTRAINT volume_parent_id_volume_id_fkey
    ;
	`)
	if err != nil {
		return err
	}

	return nil
}
