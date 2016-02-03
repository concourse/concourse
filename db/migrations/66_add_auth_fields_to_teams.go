package migrations

import "github.com/BurntSushi/migration"

func AddAuthFieldsToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
          ALTER TABLE teams
          ADD COLUMN basic_auth json null;

          ALTER TABLE teams
          ADD COLUMN github_auth json null;
      `)

	return err
}
