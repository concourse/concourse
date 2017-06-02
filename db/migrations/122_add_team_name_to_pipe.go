package migrations

import "github.com/concourse/atc/db/migration"

func AddTeamNameToPipe(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE pipes
    ADD COLUMN team_id integer REFERENCES teams (id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE pipes
		SET team_id = sub.id
		FROM (
			SELECT id
			FROM teams
			WHERE name = 'main'
		) AS sub
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipes ALTER COLUMN team_id SET NOT NULL;
	`)
	return err
}
