package migrations

import "github.com/concourse/atc/db/migration"

func AddLastScheduledToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE builds ADD COLUMN last_scheduled timestamp NOT NULL DEFAULT 'epoch';
	`)

	if err != nil {
		return err
	}

	return nil
}
