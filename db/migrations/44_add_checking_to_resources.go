package migrations

import "github.com/BurntSushi/migration"

func AddCheckingToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resources ADD COLUMN checking bool NOT NULL DEFAULT false;
	`)

	if err != nil {
		return err
	}

	return nil
}
