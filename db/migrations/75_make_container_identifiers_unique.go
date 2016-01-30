package migrations

import "github.com/BurntSushi/migration"

func MakeContainerIdentifiersUnique(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD UNIQUE
		(worker_name, resource_id, check_type, check_source, build_id, plan_id, stage)
	`)
	return err
}
