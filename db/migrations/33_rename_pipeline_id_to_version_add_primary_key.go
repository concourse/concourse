package migrations

import "github.com/concourse/atc/db/migration"

func RenamePipelineIDToVersionAddPrimaryKey(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	ALTER TABLE pipelines
		RENAME COLUMN id to version
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN id serial PRIMARY KEY;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER SEQUENCE config_id_seq RENAME TO config_version_seq
	`)
	if err != nil {
		return err
	}

	return nil
}
