package migrations

import "github.com/concourse/atc/db/migration"

func AddCreatingContainerIDAndStateToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN creating_container_id integer REFERENCES containers (id) ON DELETE SET NULL
`)
	if err != nil {
		return err
	}

	return nil
}
