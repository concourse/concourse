package migrations

import "github.com/BurntSushi/migration"

func AddImageResourceTypeAndSourceToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN image_resource_type text;
		ALTER TABLE containers ADD COLUMN image_resource_source text;
	`)
	if err != nil {
		return err
	}

	return nil
}
