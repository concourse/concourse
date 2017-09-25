package migrations

import "github.com/concourse/atc/db/migration"

func AddFailedStateToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TYPE volume_state RENAME TO volume_state_old`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TYPE volume_state AS ENUM (
			'creating',
			'created',
			'destroying',
			'failed'
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE volumes ALTER state DROP DEFAULT,
										DROP CONSTRAINT cannot_invalidate_during_initialization,
										ALTER state SET DATA TYPE volume_state USING state::text::volume_state,
										ALTER parent_state SET DATA TYPE volume_state USING parent_state::text::volume_state`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes
    ADD CONSTRAINT cannot_invalidate_during_initialization CHECK (
    (
      state IN ('created', 'destroying', 'failed') AND (
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

	_, err = tx.Exec(`ALTER TABLE volumes ALTER state SET DEFAULT 'creating'`)
	if err != nil {
		return err
	}
	return nil
}
