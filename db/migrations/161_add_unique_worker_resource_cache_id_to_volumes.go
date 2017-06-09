package migrations

import "github.com/concourse/atc/db/migration"

func AddUniqueWorkerResourceCacheIDToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE UNIQUE INDEX volumes_worker_resource_cache_unique ON volumes (worker_resource_cache_id)
	`)
	if err != nil {
		return err
	}

	return nil
}
