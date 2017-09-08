package migrations

import "github.com/concourse/atc/db/migration"

func DropUnusedBuildsCompletedIndex(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		DROP INDEX builds_completed
	`)
	if err != nil {
		return err
	}

	return nil
}
