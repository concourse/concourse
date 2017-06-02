package migrations

import "github.com/concourse/atc/db/migration"

func AddContainerIDToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN id serial PRIMARY KEY;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes ADD COLUMN container_id int;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE volumes ADD CONSTRAINT fkey_container_id FOREIGN KEY (container_id) REFERENCES containers (id);`)

	return err
}
