package migrations

import "github.com/BurntSushi/migration"

func MakeContainerWorkingDirectoryNotNull(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ALTER COLUMN working_directory SET NOT NULL,
		ALTER COLUMN working_directory SET DEFAULT ''
	`)

	return err
}
