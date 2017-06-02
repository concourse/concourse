package migrations

import "github.com/concourse/atc/db/migration"

func MakeContainersLinkToResourceIds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN resource_id INT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers c SET resource_id =
		(SELECT id from resources r where r.name = c.name 
													AND r.pipeline_id = c.pipeline_id
													AND c.type = 'check');
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		RENAME COLUMN name TO step_name;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers
		SET step_name = ''
		WHERE resource_id IS NOT NULL;
	`)
	return err
}
