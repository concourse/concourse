package migrations

import "github.com/concourse/atc/db/migration"

func AddWorkerBaseResourceTypeIdToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	  ALTER TABLE worker_base_resource_types
		ADD COLUMN id SERIAL PRIMARY KEY;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN worker_base_resource_types_id INTEGER
		REFERENCES worker_base_resource_types (id) ON DELETE SET NULL;
	`)
	if err != nil {
		return err
	}

	return nil
}
