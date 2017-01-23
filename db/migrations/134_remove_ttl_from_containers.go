package migrations

import "github.com/concourse/atc/dbng/migration"

func RemoveTTLFromContainers(tx migration.LimitedTx) error {
	var err error

	_, err = tx.Exec(`
		ALTER TABLE containers
		DROP COLUMN ttl,
		DROP COLUMN expires_at;
	`)
	if err != nil {
		return err
	}

	return nil
}
