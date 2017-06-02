package migrations

import "github.com/concourse/atc/db/migration"

func AddCompositeUniqueConstraintToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE volumes DROP CONSTRAINT volumes_handle_key;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes ADD UNIQUE (worker_name, handle);
	`)

	return err
}
