package migrations

import (
	"database/sql"
	"fmt"

	"github.com/concourse/atc/db/migration"
)

func AddWorkerResourceCacheToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE worker_resource_caches (
      id serial PRIMARY KEY,
      worker_base_resource_type_id int REFERENCES worker_base_resource_types (id) ON DELETE CASCADE,
      resource_cache_id int REFERENCES resource_caches (id) ON DELETE CASCADE
    )
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
      ALTER TABLE containers
      ADD COLUMN worker_resource_cache_id INTEGER
  		REFERENCES worker_resource_caches (id) ON DELETE SET NULL
		`)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`SELECT id, resource_cache_id, worker_name FROM containers WHERE resource_cache_id IS NOT NULL`)
	if err != nil {
		return err
	}

	defer rows.Close()

	containerWorkerResourceCaches := []containerWorkerResourceCache{}

	for rows.Next() {
		var id int
		var resourceCacheID int
		var workerName string
		err = rows.Scan(&id, &resourceCacheID, &workerName)
		if err != nil {
			return fmt.Errorf("failed to scan container id, resource_cache_id and worker_name: %s", err)
		}

		containerWorkerResourceCaches = append(containerWorkerResourceCaches, containerWorkerResourceCache{
			ID:              id,
			ResourceCacheID: resourceCacheID,
			WorkerName:      workerName,
		})
	}

	for _, cwrc := range containerWorkerResourceCaches {
		baseResourceTypeID, err := findBaseResourceTypeID(tx, cwrc.ResourceCacheID)
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
    `, baseResourceTypeID, cwrc.WorkerName).
			Scan(&workerBaseResourceTypeID)
		if err != nil {
			return err
		}

		var workerResourceCacheID int
		err = tx.QueryRow(`
				SELECT id FROM worker_resource_caches WHERE worker_base_resource_type_id = $1 AND resource_cache_id = $2
			`, workerBaseResourceTypeID, cwrc.ResourceCacheID).
			Scan(&workerResourceCacheID)
		if err != nil {
			if err != sql.ErrNoRows {
				return err
			}

			err = tx.QueryRow(`
				INSERT INTO worker_resource_caches (worker_base_resource_type_id, resource_cache_id)
		    VALUES ($1, $2)
		    RETURNING id
			`, workerBaseResourceTypeID, cwrc.ResourceCacheID).
				Scan(&workerResourceCacheID)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(`
        UPDATE containers SET worker_resource_cache_id=$1 WHERE id=$2
      `, workerResourceCacheID, cwrc.ID)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
      ALTER TABLE containers
      DROP COLUMN resource_cache_id
    `)
	if err != nil {
		return err
	}

	return nil
}

type containerWorkerResourceCache struct {
	ID              int
	ResourceCacheID int
	WorkerName      string
}

func findBaseResourceTypeID(tx migration.LimitedTx, resourceCacheID int) (int, error) {
	var innerResourceCacheID sql.NullInt64
	var baseResourceTypeID sql.NullInt64

	err := tx.QueryRow(`
    SELECT resource_cache_id, base_resource_type_id FROM resource_caches rca LEFT JOIN resource_configs rcf ON rca.resource_config_id = rcf.id WHERE rca.id=$1
  `, resourceCacheID).
		Scan(&innerResourceCacheID, &baseResourceTypeID)
	if err != nil {
		return 0, err
	}

	if baseResourceTypeID.Valid {
		return int(baseResourceTypeID.Int64), nil
	}

	if innerResourceCacheID.Valid {
		return findBaseResourceTypeID(tx, int(innerResourceCacheID.Int64))
	}

	return 0, nil
}
