package migrations

import "github.com/concourse/atc/db/migration"

func AddTeamIDToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE builds
    ADD COLUMN team_id integer REFERENCES teams (id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE builds
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
		ALTER TABLE builds ALTER COLUMN team_id SET NOT NULL;
	`)
	return err
}
