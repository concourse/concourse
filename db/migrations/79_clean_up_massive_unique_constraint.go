package migrations

import "github.com/concourse/atc/db/migration"

func CleanUpMassiveUniqueConstraint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		DROP CONSTRAINT IF EXISTS containers_worker_name_resource_id_check_type_check_source__key
	`)
	return err
}
