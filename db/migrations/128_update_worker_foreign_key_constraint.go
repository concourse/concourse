package migrations

import "github.com/concourse/atc/db/migration"

func UpdateWorkerForeignKeyConstraint(tx migration.LimitedTx) error {
	var err error

	_, err = tx.Exec(`
		ALTER TABLE volumes
		DROP CONSTRAINT volumes_worker_name_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes
		ADD CONSTRAINT volumes_worker_name_fkey
		FOREIGN KEY (worker_name)
		REFERENCES workers (name) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		DROP CONSTRAINT containers_worker_name_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD CONSTRAINT containers_worker_name_fkey
		FOREIGN KEY (worker_name)
		REFERENCES workers (name) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	return nil
}
