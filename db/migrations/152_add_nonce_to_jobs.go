package migrations

import "github.com/concourse/atc/db/migration"

func AddNonceToJobs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE jobs
		ADD COLUMN nonce text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs
		ALTER COLUMN config TYPE text;
`)
	if err != nil {
		return err
	}

	return nil
}
