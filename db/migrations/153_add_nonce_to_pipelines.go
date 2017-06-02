package migrations

import "github.com/concourse/atc/db/migration"

func AddNonceToPipelines(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines
		ADD COLUMN nonce text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipelines
		ALTER COLUMN config TYPE text;
`)
	if err != nil {
		return err
	}

	return nil
}
