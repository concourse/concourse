package migrations

import "github.com/concourse/atc/dbng/migration"

func AddAdminToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
          ALTER TABLE teams
          ADD COLUMN admin bool DEFAULT false;
      `)

	return err
}
