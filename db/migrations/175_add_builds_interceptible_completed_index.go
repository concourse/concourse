package migrations

import "github.com/concourse/atc/db/migration"

func AddBuildsInterceptibleCompletedIndex(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE INDEX builds_interceptible_completed ON builds (interceptible, completed);
	`)
	if err != nil {
		return err
	}

	return nil
}
