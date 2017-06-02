package migrations

import "github.com/concourse/atc/db/migration"

func AddNameToPipelines(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN name text;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipelines ADD CONSTRAINT constraint_pipelines_name_unique UNIQUE (name);
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE pipelines
		SET name = 'main';
	`)

	if err != nil {
		return err
	}

	return err
}
