package migrations

import "github.com/concourse/atc/dbng/migration"

func AddPathToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		ADD COLUMN path text
	`)
	return err
}
