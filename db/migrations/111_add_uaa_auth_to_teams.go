package migrations

import "github.com/concourse/atc/db/migration"

func AddUAAAuthToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE teams
    ADD COLUMN uaa_auth json null;
	`)
	return err
}
