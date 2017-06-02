package migrations

import "github.com/concourse/atc/db/migration"

func AddCheckTypeAndCheckSourceToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN check_type text;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers ADD COLUMN check_source text;
	`)

	return err
}
