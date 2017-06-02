package migrations

import "github.com/concourse/atc/db/migration"

func MakeVolumesExpiresAtNullable(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		ALTER COLUMN expires_at DROP NOT NULL;
	`)
	return err
}
