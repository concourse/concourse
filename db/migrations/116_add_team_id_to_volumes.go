package migrations

import "github.com/concourse/atc/db/migration"

func AddTeamIDToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE volumes
      ADD COLUMN team_id integer
      REFERENCES teams (id)
      ON DELETE SET NULL;
	`)

	return err
}
