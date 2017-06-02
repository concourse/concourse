package migrations

import "github.com/concourse/atc/db/migration"

func MakeContainersLinkToPipelineIds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN pipeline_id INT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers c SET pipeline_id =
		(SELECT id FROM pipelines p WHERE p.name = c.pipeline_name);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		DROP COLUMN pipeline_name;
	`)
	return err
}
