package migrations

import "github.com/concourse/atc/db/migration"

func AddPlatformAndTagsToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE workers ADD COLUMN platform text`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE workers ADD COLUMN tags text`)
	if err != nil {
		return err
	}

	return nil
}
