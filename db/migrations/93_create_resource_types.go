package migrations

import "github.com/concourse/atc/db/migration"

func CreateResourceTypes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE resource_types (
			id serial PRIMARY KEY,
			pipeline_id int REFERENCES pipelines (id) ON DELETE CASCADE,
			name text NOT NULL,
			type text NOT NULL,
			version text,
			UNIQUE (pipeline_id, name)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN resource_type_version text
	`)
	return err
}
