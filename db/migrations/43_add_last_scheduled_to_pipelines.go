package migrations

import "github.com/concourse/atc/db/migration"

func AddLastScheduledToPipelines(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN last_scheduled timestamp NOT NULL DEFAULT 'epoch';
	`)

	if err != nil {
		return err
	}

	return nil
}
