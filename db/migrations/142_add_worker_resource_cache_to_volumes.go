package migrations

import (
	"database/sql"
	"fmt"

	"github.com/concourse/atc/db/migration"
)

func AddWorkerResourceCacheToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
      ALTER TABLE volumes
      ADD COLUMN worker_resource_cache_id INTEGER
  		REFERENCES worker_resource_caches (id) ON DELETE SET NULL
		`)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`SELECT id, resource_cache_id, worker_name FROM volumes WHERE resource_cache_id IS NOT NULL`)
	if err != nil {
		return err
	}

	defer rows.Close()

	volumeWorkerResourceCaches := []volumeWorkerResourceCache{}

	for rows.Next() {
		var id int
		var resourceCacheID int
		var workerName string
		err = rows.Scan(&id, &resourceCacheID, &workerName)
		if err != nil {
			return fmt.Errorf("failed to scan volume id, resource_cache_id and worker_name: %s", err)
		}

		volumeWorkerResourceCaches = append(volumeWorkerResourceCaches, volumeWorkerResourceCache{
			ID:              id,
			ResourceCacheID: resourceCacheID,
			WorkerName:      workerName,
		})
	}

	for _, vwrc := range volumeWorkerResourceCaches {
		baseResourceTypeID, err := findBaseResourceTypeID(tx, vwrc.ResourceCacheID)
		if err != nil {
			return err
		}
		if baseResourceTypeID == 0 {
			// most likely resource cache was garbage collected
			// keep worker_base_resource_type_id as null, so that gc can remove this container
			continue
		}

		var workerBaseResourceTypeID int
		err = tx.QueryRow(`
		      SELECT id FROM worker_base_resource_types WHERE base_resource_type_id=$1 AND worker_name=$2
		    `, baseResourceTypeID, vwrc.WorkerName).
			Scan(&workerBaseResourceTypeID)
		if err != nil {
			return err
		}

		var workerResourceCacheID int
		err = tx.QueryRow(`
				SELECT id FROM worker_resource_caches WHERE worker_base_resource_type_id = $1 AND resource_cache_id = $2
			`, workerBaseResourceTypeID, vwrc.ResourceCacheID).
			Scan(&workerResourceCacheID)
		if err != nil {
			if err != sql.ErrNoRows {
				return err
			}

			err = tx.QueryRow(`
				INSERT INTO worker_resource_caches (worker_base_resource_type_id, resource_cache_id)
		    VALUES ($1, $2)
		    RETURNING id
			`, workerBaseResourceTypeID, vwrc.ResourceCacheID).
				Scan(&workerResourceCacheID)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(`
        UPDATE volumes SET worker_resource_cache_id=$1 WHERE id=$2
      `, workerResourceCacheID, vwrc.ID)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes
    DROP COLUMN resource_cache_id,
		ADD CONSTRAINT cannot_invalidate_during_initialization CHECK (
			(
				state IN ('created', 'destroying') AND (
					(
						worker_resource_cache_id IS NULL
					) AND (
						worker_base_resource_type_id IS NULL
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

type volumeWorkerResourceCache struct {
	ID              int
	ResourceCacheID int
	WorkerName      string
}
