package migrations

import (
	"github.com/concourse/atc/db/migration"
)

func RemoveLastTrackedFromBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE builds
    DROP COLUMN last_tracked
  `)
	if err != nil {
		return err
	}

	return nil
}
