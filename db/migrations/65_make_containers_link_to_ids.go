package migrations

import "github.com/BurntSushi/migration"

func MakeContainersLinkToIds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN pipeline_id INT REFERENCES pipelines(id) NULL;

		UPDATE containers c SET pipeline_id =
		(SELECT id FROM pipelines p where p.name = c.pipeline_name);

		ALTER TABLE containers
		DROP COLUMN pipeline_name;

		ALTER TABLE containers
		ADD COLUMN resource_id INT REFERENCES resources(id) NULL;

		UPDATE containers c SET resource_id =
		(SELECT id from resources r where r.name = c.name);

		ALTER TABLE containers
		RENAME COLUMN name TO step_name;

		UPDATE containers
		SET step_name = ''
		WHERE resource_id IS NOT NULL;

		ALTER TABLE containers
		ADD CONSTRAINT containers_build_id_fk FOREIGN KEY (build_id)
		               REFERENCES builds(id);

		ALTER TABLE containers
		ALTER COLUMN build_id DROP NOT NULL;

		UPDATE containers SET build_id = NULL
		WHERE build_id = 0;

		ALTER TABLE workers
		ADD COLUMN id BIGSERIAL PRIMARY KEY;

		ALTER TABLE containers
		ADD COLUMN worker_id INT REFERENCES workers(id) NULL;

		UPDATE containers c SET worker_id =
		(select id from workers w where w.name = c.worker_name);

		ALTER TABLE containers
		DROP COLUMN worker_name;
	`)
	return err
}
