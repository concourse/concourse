package migrations

import "github.com/concourse/atc/db/migration"

func ResetCheckOrder(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	UPDATE versioned_resources
	SET check_order = id
	WHERE check_order != id
	`)
	return err
}
