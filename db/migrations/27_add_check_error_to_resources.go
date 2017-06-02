package migrations

import "github.com/concourse/atc/db/migration"

func AddCheckErrorToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE resources ADD COLUMN check_error text NULL`)

	return err
}
