package migrations

import "github.com/concourse/atc/db/migration"

func AddLastCheckedToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resources ADD COLUMN last_checked timestamp NOT NULL DEFAULT 'epoch';
	`)

	if err != nil {
		return err
	}

	return nil
}
