package migrations

import "github.com/BurntSushi/migration"

func AddAttemptsToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN attempts text NULL;
	`)
	return err
}
