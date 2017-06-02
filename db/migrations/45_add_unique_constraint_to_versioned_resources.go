package migrations

import "github.com/concourse/atc/db/migration"

func AddUniqueConstraintToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		WITH distinct_vrs AS (
			SELECT DISTINCT ON (resource_id, type, version) id
			FROM versioned_resources
		), deleted_outputs AS (
			DELETE FROM build_outputs WHERE versioned_resource_id NOT IN (SELECT id FROM distinct_vrs)
		), deleted_inputs AS (
			DELETE FROM build_inputs WHERE versioned_resource_id NOT IN (SELECT id FROM distinct_vrs)
		)
		DELETE FROM versioned_resources WHERE id NOT IN (SELECT id FROM distinct_vrs)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX versioned_resources_resource_id_type_version
		ON versioned_resources (resource_id, type, version)
	`)
	if err != nil {
		return err
	}

	return nil
}
