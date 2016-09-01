package migrations

import "github.com/BurntSushi/migration"

func AddGenericOAuthToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE teams
    ADD COLUMN genericoauth_auth json null;
	`)
	return err
}
