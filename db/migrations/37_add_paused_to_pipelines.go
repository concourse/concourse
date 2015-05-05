package migrations

import "github.com/BurntSushi/migration"

func AddPausedToPipelines(tx migration.LimitedTx) error {

	_, err := tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN paused boolean DEFAULT(false);
`)

	return err

}
