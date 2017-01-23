package migrations

import "github.com/concourse/atc/dbng/migration"

func AddUserToContainer(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	  ALTER TABLE containers
		ADD COLUMN process_user text DEFAULT '';
	`)
	return err
}
