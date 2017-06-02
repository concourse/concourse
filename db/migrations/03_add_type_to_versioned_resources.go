package migrations

import "github.com/concourse/atc/db/migration"

func AddTypeToVersionedResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE versioned_resources
		ADD COLUMN type text,
		DROP CONSTRAINT versioned_resources_resource_name_version_key,
		ADD UNIQUE (resource_name, type, version);
	`)
	if err != nil {
		return err
	}

	return nil
}
