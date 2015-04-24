package migrations

import "github.com/BurntSushi/migration"

func AddPipelineIDToResources(tx migration.LimitedTx) error {

	_, err := tx.Exec(`
		ALTER TABLE versioned_resources DROP CONSTRAINT versioned_resources_resource_name_fkey;

		ALTER TABLE resources DROP CONSTRAINT resources_pkey;

		ALTER TABLE resources ADD COLUMN id serial PRIMARY KEY;

		ALTER TABLE resources ADD COLUMN pipeline_id int REFERENCES pipelines (id);

		UPDATE resources
		SET pipeline_id = (
			SELECT id
			FROM pipelines
			WHERE name = 'main'
			LIMIT 1
		);

		ALTER TABLE resources ADD CONSTRAINT unique_pipeline_id_name UNIQUE (pipeline_id, name);

		ALTER TABLE resources ALTER COLUMN pipeline_id SET NOT NULL;
		ALTER TABLE resources ALTER COLUMN name SET NOT NULL;


		ALTER TABLE versioned_resources ADD COLUMN resource_id int;

		UPDATE versioned_resources
		SET resource_id = resources.id
		FROM resources
		WHERE versioned_resources.resource_name = resources.name;

		ALTER TABLE versioned_resources ADD CONSTRAINT fkey_resource_id FOREIGN KEY (resource_id) REFERENCES resources (id);

		ALTER TABLE versioned_resources DROP COLUMN resource_name;
`)

	return err

}
