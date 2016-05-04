package migrations

import "github.com/BurntSushi/migration"

func AddMissingInputReasonsToBuildPreparation(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE build_preparation ADD COLUMN missing_input_reasons json DEFAULT '{}';
	`)
	if err != nil {
		return err
	}

	return nil
}
