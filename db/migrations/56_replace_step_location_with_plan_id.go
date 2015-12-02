package migrations

import "github.com/BurntSushi/migration"

func ReplaceStepLocationWithPlanID(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE containers DROP COLUMN step_location;
    ALTER TABLE containers ADD COLUMN plan_id text;
	`)

	return err
}
