package migrations

import "github.com/concourse/atc/db/migration"

func AddIndexesForBuildCollector(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE INDEX builds_completed ON builds (completed)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX builds_status ON builds (status)`)
	if err != nil {
		return err
	}

	return nil
}
