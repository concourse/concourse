package migrations

import "github.com/concourse/atc/db/migration"

func ReplaceStepLocationWithPlanID(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE containers DROP COLUMN step_location;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE containers ADD COLUMN plan_id text;
	`)

	return err
}
