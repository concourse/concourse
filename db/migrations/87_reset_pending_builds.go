package migrations

import "github.com/concourse/atc/db/migration"

func ResetPendingBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	UPDATE builds
	SET scheduled = false
	WHERE scheduled = true AND status = 'pending'
	`)
	return err
}
