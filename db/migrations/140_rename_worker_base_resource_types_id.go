package migrations

import (
	"github.com/concourse/atc/db/migration"
)

func RenameWorkerBaseResourceTypesId(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
      ALTER TABLE containers
      RENAME COLUMN worker_base_resource_types_id TO worker_base_resource_type_id
		`)
	if err != nil {
		return err
	}

	return nil
}
