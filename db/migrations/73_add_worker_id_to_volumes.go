package migrations

import "github.com/concourse/atc/db/migration"

func AddWorkerIDToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ADD COLUMN worker_id integer;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE volumes v set worker_id =
		(SELECT id from workers w where w.name = v.worker_name);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes ALTER COLUMN worker_id SET NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX volumes_worker_id ON volumes (worker_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes DROP CONSTRAINT volumes_worker_name_handle_key;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes ADD UNIQUE (worker_id, handle);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes DROP COLUMN worker_name;
	`)
	return err
}
