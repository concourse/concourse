package migrations

import "github.com/BurntSushi/migration"

func AddTeamIDToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE workers
      ADD COLUMN team_id integer
			REFERENCES teams (id)
			ON DELETE CASCADE;
	`)

	return err
}
