package migrations

import "github.com/concourse/atc/db/migration"

func AddTeamsColumnToPipelinesAndTeamsTable(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE teams (
      id serial PRIMARY KEY,
			name text NOT NULL,
      CONSTRAINT constraint_teams_name_unique UNIQUE (name)
    )
  `)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO teams (name) VALUES ('main')
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN team_id integer REFERENCES teams (id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE pipelines
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
		ALTER TABLE pipelines ALTER COLUMN team_id SET NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX pipelines_team_id ON pipelines (team_id);
	`)
	return err
}
