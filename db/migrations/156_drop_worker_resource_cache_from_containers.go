package migrations

import "github.com/concourse/atc/db/migration"

func DropWorkerResourceCacheFromContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		DROP COLUMN worker_resource_cache_id
`)
	if err != nil {
		return err
	}

	return nil
}
