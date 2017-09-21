package migrations

import "github.com/concourse/atc/db/migration"

func AddUniqueIndexesToMaterializedViews(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE UNIQUE INDEX latest_completed_builds_per_job_id ON latest_completed_builds_per_job (id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX next_builds_per_job_id ON next_builds_per_job (id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX transition_builds_per_job_id ON transition_builds_per_job (id)
	`)
	if err != nil {
		return err
	}

	return nil
}
