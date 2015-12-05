package migrations

import "github.com/BurntSushi/migration"

func MakeContainerWorkingDirectoryNotNull(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		BEGIN WORK;
		LOCK TABLE containers IN SHARE ROW EXCLUSIVE MODE;

		UPDATE containers SET working_directory = '' WHERE working_directory IS NULL;

		ALTER TABLE containers
		ALTER COLUMN working_directory SET DEFAULT '',
		ALTER COLUMN working_directory SET NOT NULL;

		COMMIT WORK;
	`)

	return err
}
