package migrations

import "github.com/concourse/atc/db/migration"

func RemoveWorkerIds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ADD COLUMN worker_name text;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE volumes v set worker_name =
		(SELECT name from workers w where w.id = v.worker_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM volumes WHERE worker_name IS NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes ALTER COLUMN worker_name SET NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX volumes_worker_name ON volumes (worker_name);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes DROP CONSTRAINT volumes_worker_id_handle_key;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes ADD UNIQUE (worker_name, handle);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes DROP COLUMN worker_id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN worker_name TEXT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers c SET worker_name =
		(select name from workers w where w.id = c.worker_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		DROP COLUMN worker_id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers
		DROP COLUMN id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers
		ADD PRIMARY KEY (name);
	`)
	return err
}
