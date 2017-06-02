package migrations

import "github.com/concourse/atc/db/migration"

func MakeContainersBuildIdsNullable(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ALTER COLUMN build_id DROP NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers SET build_id = NULL
		WHERE build_id = 0;
	`)
	return err
}
