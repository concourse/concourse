package migrations

import "github.com/concourse/atc/db/migration"

func AddStageToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TYPE container_stage AS ENUM (
      'check',
      'get',
      'run'
    )
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers ADD COLUMN stage container_stage NOT NULL DEFAULT 'run';
	`)
	return err
}
