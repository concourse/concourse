package migrations

import "github.com/concourse/atc/db/migration"

func AddStateToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TYPE worker_state AS ENUM (
			'running',
			'stalled',
			'landing',
			'landed',
			'retiring'
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN state worker_state DEFAULT 'running' NOT NULL,
		ALTER COLUMN addr DROP NOT NULL
	`)
	return err
}
