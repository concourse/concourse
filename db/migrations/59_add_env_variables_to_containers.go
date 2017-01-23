package migrations

import "github.com/concourse/atc/dbng/migration"

func AddEnvVariablesToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN env_variables text NOT NULL DEFAULT '[]';
	`)

	return err
}
