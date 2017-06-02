package migrations

import "github.com/concourse/atc/db/migration"

func RemoveTTLFromVolumes(tx migration.LimitedTx) error {
	var err error

	_, err = tx.Exec(`
		ALTER TABLE volumes
		DROP COLUMN ttl,
		DROP COLUMN expires_at;
	`)
	if err != nil {
		return err
	}

	return nil
}
