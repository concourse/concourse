package migrations

import "github.com/concourse/atc/db/migration"

func AddLastTrackedToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE builds ADD COLUMN last_tracked timestamp NOT NULL DEFAULT 'epoch';
	`)

	if err != nil {
		return err
	}

	return nil
}
