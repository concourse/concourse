package migrations

import "github.com/concourse/atc/db/migration"

func CascadeTeamDeletesOnPipes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipes DROP CONSTRAINT pipes_team_id_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipes ADD CONSTRAINT pipes_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams (id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	return nil
}
