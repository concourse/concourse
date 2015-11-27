package migrations

import "github.com/BurntSushi/migration"

func AddWorkingDirectoryToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN working_directory text;
	`)

	return err
}
