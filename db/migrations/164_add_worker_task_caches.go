package migrations

import "github.com/concourse/atc/db/migration"

func AddWorkerTaskCaches(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE worker_task_caches (
      id serial PRIMARY KEY,
      worker_name text REFERENCES workers (name) ON DELETE CASCADE,
      job_id int REFERENCES jobs (id) ON DELETE CASCADE,
      step_name text NOT NULL,
      path text NOT NULL
    )
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
      ALTER TABLE volumes
      ADD COLUMN worker_task_cache_id int REFERENCES worker_task_caches (id) ON DELETE SET NULL,
      DROP CONSTRAINT cannot_invalidate_during_initialization
		`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes
    ADD CONSTRAINT cannot_invalidate_during_initialization CHECK (
    (
      state IN ('created', 'destroying') AND (
      (
        worker_resource_cache_id IS NULL
      ) AND (
        worker_base_resource_type_id IS NULL
      ) AND (
        worker_task_cache_id IS NULL
      ) AND (
        container_id IS NULL
      )
    )
      ) OR (
        (
          worker_resource_cache_id IS NOT NULL
        ) OR (
          worker_base_resource_type_id IS NOT NULL
        ) OR (
          worker_task_cache_id IS NOT NULL
        ) OR (
          container_id IS NOT NULL
        )
      )
    )
  `)
	if err != nil {
		return err
	}

	return nil
}
