package migrations

import "github.com/concourse/atc/db/migration"

func AddPausedToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE resources ADD COLUMN paused boolean DEFAULT(false)`)

	return err
}
