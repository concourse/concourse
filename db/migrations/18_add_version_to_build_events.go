package migrations

import "github.com/concourse/atc/db/migration"

func AddVersionToBuildEvents(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE build_events ADD COLUMN version text`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`UPDATE build_events	SET version = '1.0'`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE build_events ALTER COLUMN version SET NOT NULL`)
	if err != nil {
		return err
	}

	return nil
}
