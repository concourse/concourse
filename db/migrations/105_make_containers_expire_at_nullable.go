package migrations

import "github.com/concourse/atc/db/migration"

func MakeContainersExpiresAtNullable(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ALTER COLUMN expires_at DROP NOT NULL;
	`)
	return err
}
