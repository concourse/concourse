package migrations

import "github.com/concourse/atc/db/migration"

func AddTTLToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN ttl text NOT NULL DEFAULT 0;
	`)
	if err != nil {
		return err
	}

	return nil
}
