package migrations

import "github.com/concourse/atc/dbng/migration"

func AddSourceHashToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
  ALTER TABLE resources
  ADD COLUMN source_hash text NOT NULL
`)
	if err != nil {
		return err
	}

	return nil
}
