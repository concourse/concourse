package migrations

import "github.com/BurntSushi/migration"

func AddVolumeParentIdForeignKey(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		ADD CONSTRAINT volume_parent_id_volume_id_fkey
		FOREIGN KEY (parent_id)
		REFERENCES volumes(id)
		ON DELETE RESTRICT
    ;
	`)
	if err != nil {
		return err
	}

	return nil
}
