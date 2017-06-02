package migrations

import "github.com/concourse/atc/db/migration"

func AddTeamIDToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE containers
      ADD COLUMN team_id integer
			REFERENCES teams (id)
			ON DELETE SET NULL;
	`)

	return err
}
