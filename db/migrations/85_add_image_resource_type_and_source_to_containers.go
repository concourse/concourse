package migrations

import "github.com/concourse/atc/db/migration"

func AddImageResourceTypeAndSourceToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN image_resource_type text
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers ADD COLUMN image_resource_source text
	`)
	return err
}
