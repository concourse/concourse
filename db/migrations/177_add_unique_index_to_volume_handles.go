package migrations

import "github.com/concourse/atc/db/migration"

func AddUniqueIndexToVolumeHandles(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE UNIQUE INDEX volumes_handle ON volumes (handle)
	`)
	if err != nil {
		return err
	}

	return nil
}
