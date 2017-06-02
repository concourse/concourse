package migrations

import "github.com/concourse/atc/db/migration"

func AddHostPathVersionToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		ADD COLUMN host_path_version text;
`)
	return err
}
