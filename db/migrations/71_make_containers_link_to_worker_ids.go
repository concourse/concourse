package migrations

import "github.com/concourse/atc/db/migration"

func MakeContainersLinkToWorkerIds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN id BIGSERIAL PRIMARY KEY;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN worker_id INT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers c SET worker_id =
		(select id from workers w where w.name = c.worker_name);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		DROP COLUMN worker_name;
	`)
	return err
}
