package migrations

import (
	"github.com/concourse/atc/db/migration"
)

func ChangeVolumeBaseResourceTypeToWorkerBaseResourceType(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
      ALTER TABLE volumes
      ADD COLUMN worker_base_resource_type_id int REFERENCES worker_base_resource_types (id) ON DELETE SET NULL
		`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
      UPDATE volumes v
      SET worker_base_resource_type_id=(SELECT id FROM worker_base_resource_types w WHERE v.worker_name = w.worker_name AND v.base_resource_type_id = w.base_resource_type_id)
      WHERE v.base_resource_type_id IS NOT NULL
    `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes
    DROP COLUMN base_resource_type_id
  `)
	if err != nil {
		return err
	}

	return nil
}
