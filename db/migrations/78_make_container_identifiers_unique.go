package migrations

import "github.com/concourse/atc/db/migration"

func MakeContainerIdentifiersUnique(tx migration.LimitedTx) error {
	// This migration used to run the following, which led to errors from
	// postgres as the resulting data for maintaining the index was too large:

	// _, err := tx.Exec(`
	// 	ALTER TABLE containers ADD UNIQUE
	// 	(worker_name, resource_id, check_type, check_source, build_id, plan_id, stage)
	// `)
	// return err

	// The error was:
	//
	//   index row size 3528 exceeds maximum 2712 for index
	//   "containers_worker_name_resource_id_check_type_check_source__key"`

	return nil
}
