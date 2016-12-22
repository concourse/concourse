package migrations

import "github.com/BurntSushi/migration"

func AddInterruptibleToJob(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE jobs
			ADD COLUMN interruptible bool NOT NULL DEFAULT false
	`)
	if err != nil {
		return err
	}

	return nil
}
