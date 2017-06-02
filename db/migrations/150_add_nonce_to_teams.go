package migrations

import "github.com/concourse/atc/db/migration"

func AddNonceToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE teams
		ADD COLUMN nonce text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE teams
		ALTER COLUMN auth TYPE text;
`)
	if err != nil {
		return err
	}

	return nil
}
