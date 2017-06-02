package migrations

import "github.com/concourse/atc/db/migration"

func AddWorkingDirectoryToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN working_directory text;
	`)

	return err
}
