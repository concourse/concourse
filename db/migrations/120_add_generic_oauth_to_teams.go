package migrations

import "github.com/concourse/atc/db/migration"

func AddGenericOAuthToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE teams
    ADD COLUMN genericoauth_auth json null;
	`)
	return err
}
