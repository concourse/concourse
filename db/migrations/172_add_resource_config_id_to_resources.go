package migrations

import "github.com/concourse/atc/db/migration"

func AddResourceConfigIDToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE resources
		DROP COLUMN source_hash,
		ADD COLUMN resource_config_id integer REFERENCES resource_configs (id) ON DELETE SET NULL
  `)
	if err != nil {
		return err
	}

	return nil
}
