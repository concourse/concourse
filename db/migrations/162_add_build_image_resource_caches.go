package migrations

import "github.com/concourse/atc/db/migration"

func AddBuildImageResourceCaches(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE build_image_resource_caches (
			resource_cache_id integer REFERENCES resource_caches (id) ON DELETE RESTRICT,
			build_id integer NOT NULL REFERENCES builds (id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DROP TABLE image_resource_versions
	`)
	if err != nil {
		return err
	}

	return nil
}
