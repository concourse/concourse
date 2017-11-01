package migrations

import "github.com/concourse/atc/db/migration"

func CreateResourceSpaces(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE resource_spaces (
			id serial PRIMARY KEY,
			resource_id int REFERENCES resources (id) ON DELETE CASCADE,
			name text NOT NULL,
			UNIQUE (resource_id, name)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO resource_spaces(id, resource_id, name) SELECT id, id, 'default' from resources;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources RENAME resource_id TO resource_space_id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources DROP CONSTRAINT fkey_resource_id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources ADD CONSTRAINT resource_space_id_fkey FOREIGN KEY (resource_space_id) REFERENCES resource_spaces (id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER INDEX versioned_resources_resource_id_type_version RENAME TO versioned_resources_resource_space_id_type_version;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER INDEX versioned_resources_resource_id_idx RENAME TO versioned_resources_resource_space_id_idx;
	`)
	return err
}
