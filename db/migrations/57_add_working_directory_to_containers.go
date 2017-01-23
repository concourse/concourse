package migrations

import "github.com/concourse/atc/dbng/migration"

func AddWorkingDirectoryToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN working_directory text;
	`)

	return err
}
