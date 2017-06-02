package migrations

import "github.com/concourse/atc/db/migration"

func AddPipelineIDToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE versioned_resources DROP CONSTRAINT versioned_resources_resource_name_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources DROP CONSTRAINT resources_pkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ADD COLUMN id serial PRIMARY KEY;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ADD COLUMN pipeline_id int REFERENCES pipelines (id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE resources
		SET pipeline_id = (
			SELECT id
			FROM pipelines
			WHERE name = 'main'
			LIMIT 1
		);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ADD CONSTRAINT unique_pipeline_id_name UNIQUE (pipeline_id, name);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ALTER COLUMN pipeline_id SET NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ALTER COLUMN name SET NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources ADD COLUMN resource_id int;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE versioned_resources
		SET resource_id = resources.id
		FROM resources
		WHERE versioned_resources.resource_name = resources.name;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources ADD CONSTRAINT fkey_resource_id FOREIGN KEY (resource_id) REFERENCES resources (id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources DROP COLUMN resource_name;
	`)

	return err

}
