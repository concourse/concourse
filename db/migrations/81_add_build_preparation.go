package migrations

import "github.com/concourse/atc/db/migration"

func AddBuildPreparation(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	CREATE TABLE build_preparation (
		build_id integer PRIMARY KEY REFERENCES builds (id) ON DELETE CASCADE,
		paused_pipeline text DEFAULT 'unknown',
		paused_job text DEFAULT 'unknown',
		max_running_builds text DEFAULT 'unknown',
		inputs json DEFAULT '{}',
		completed bool DEFAULT false
	)
	`)
	return err
}
