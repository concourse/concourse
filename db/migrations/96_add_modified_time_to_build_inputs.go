package migrations

import "github.com/BurntSushi/migration"

func AddModifiedTimeToBuildInputs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE build_inputs
		ADD COLUMN modified_time timestamp NOT NULL DEFAULT now();
`)
	return err
}
