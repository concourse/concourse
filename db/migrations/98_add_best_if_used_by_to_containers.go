package migrations

import "github.com/BurntSushi/migration"

func AddBestIfUsedByToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN best_if_used_by timestamp;
`)
	return err
}
