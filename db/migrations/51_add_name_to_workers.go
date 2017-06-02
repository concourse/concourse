package migrations

import "github.com/concourse/atc/db/migration"

func AddNameToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers ADD COLUMN name text;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE workers
		SET name = workers.addr;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers ADD CONSTRAINT constraint_workers_name_unique UNIQUE (name);
	`)

	if err != nil {
		return err
	}

	return err
}
