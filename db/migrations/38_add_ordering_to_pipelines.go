package migrations

import "github.com/concourse/atc/db/migration"

func AddOrderingToPipelines(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN ordering int DEFAULT(0) NOT NULL
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
			UPDATE pipelines
			SET ordering = id
	`)

	return err
}
