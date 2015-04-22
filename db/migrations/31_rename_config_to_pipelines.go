package migrations

import "github.com/BurntSushi/migration"

func RenameConfigToPipelines(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	ALTER TABLE config
    RENAME TO pipelines
	`)
	return err
}
