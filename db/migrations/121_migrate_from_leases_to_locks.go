package migrations

import "github.com/concourse/atc/db/migration"

func MigrateFromLeasesToLocks(tx migration.LimitedTx) error {
	_, err := tx.Exec(`DROP TABLE leases`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs
		DROP COLUMN resource_check_finished_at,
		ADD COLUMN resource_checking bool NOT NULL DEFAULT false
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources DROP COLUMN checking
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resource_types
		DROP COLUMN checking
	`)
	if err != nil {
		return err
	}

	return nil
}
