package migrations

import "github.com/concourse/atc/db/migration"

func AddConfig(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE config (config text NOT NULL)`)
	if err != nil {
		return err
	}

	return nil
}
