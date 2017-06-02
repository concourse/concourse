package migrations

import "github.com/concourse/atc/db/migration"

func AddOnDeleteSetNullToFKeyContainerId(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes DROP CONSTRAINT fkey_container_id;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE volumes ADD CONSTRAINT fkey_container_id FOREIGN KEY (container_id) REFERENCES containers (id) ON DELETE SET NULL;`)

	return err
}
