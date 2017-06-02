package migrations

import "github.com/concourse/atc/db/migration"

func AddOrderToVersionedResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE versioned_resources 
		ADD COLUMN check_order int
		DEFAULT 0 NOT NULL;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE versioned_resources
		SET check_order = id;
	`)

	return err
}
