package migrations

import "github.com/concourse/atc/db/migration"

func AddUniqueWorkerResourceCacheIDToVolumes(tx migration.LimitedTx) error {
	// handle initialized volumes first to avoid
	// 'cannot_invalidate_during_initialization'
	_, err := tx.Exec(`
		WITH distinct_vols AS (
			SELECT DISTINCT ON (worker_resource_cache_id) id
			FROM volumes
			WHERE worker_resource_cache_id IS NOT NULL
		)
		UPDATE volumes
		SET worker_resource_cache_id = NULL, state = 'destroying'
		WHERE worker_resource_cache_id IS NOT NULL
		AND state = 'creating'
		AND id NOT IN (SELECT id FROM distinct_vols)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		WITH distinct_vols AS (
			SELECT DISTINCT ON (worker_resource_cache_id) id
			FROM volumes
			WHERE worker_resource_cache_id IS NOT NULL
		)
		UPDATE volumes
		SET worker_resource_cache_id = NULL
		WHERE worker_resource_cache_id IS NOT NULL
		AND id NOT IN (SELECT id FROM distinct_vols)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX volumes_worker_resource_cache_unique
		ON volumes (worker_resource_cache_id)
	`)
	if err != nil {
		return err
	}

	return nil
}
