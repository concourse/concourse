package migrations

import "github.com/concourse/atc/db/migration"

func AddExplicitToBuildOutputs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE build_outputs ADD COLUMN explicit bool NOT NULL DEFAULT false
	`)

	if err != nil {
		return err
	}

	return nil
}
