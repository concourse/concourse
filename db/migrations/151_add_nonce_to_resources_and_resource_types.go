package migrations

import "github.com/concourse/atc/db/migration"

func AddNonceToResourcesAndResourceTypes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resources
		ADD COLUMN nonce text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources
		ALTER COLUMN config TYPE text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resource_types
		ADD COLUMN nonce text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resource_types
		ALTER COLUMN config TYPE text;
`)
	if err != nil {
		return err
	}

	return nil
}
