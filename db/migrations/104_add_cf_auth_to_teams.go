package migrations

import "github.com/BurntSushi/migration"

func AddCFAuthToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE teams
    ADD COLUMN cf_auth json null;
	`)
	return err
}
