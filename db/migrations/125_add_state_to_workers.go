package migrations

import "github.com/BurntSushi/migration"

func AddStateToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TYPE worker_state AS ENUM (
			'running',
			'stalled',
			'landing'
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN state worker_state DEFAULT 'running' NOT NULL
	`)
	return err
}
