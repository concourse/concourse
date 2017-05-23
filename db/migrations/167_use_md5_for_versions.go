package migrations

import "github.com/concourse/atc/db/migration"

func UseMd5ForVersions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		DROP INDEX versioned_resources_resource_id_type_version;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX versioned_resources_resource_id_type_version
		ON versioned_resources (resource_id, type, md5(version));
`)
	if err != nil {
		return err
	}

	return nil
}
