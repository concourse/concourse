package migrations

import "github.com/concourse/atc/db/migration"

func ReplaceCreatingContainerIDWithImageCheckForContainerIDAndImageGetForContainerID(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN image_check_container_id integer REFERENCES containers (id) ON DELETE SET NULL,
		ADD COLUMN image_get_container_id integer REFERENCES containers (id) ON DELETE SET NULL,
		DROP COLUMN creating_container_id
`)
	if err != nil {
		return err
	}

	return nil
}
