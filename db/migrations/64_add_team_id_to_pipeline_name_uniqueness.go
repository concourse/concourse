package migrations

import "github.com/concourse/atc/dbng/migration"

func AddTeamIDToPipelineNameUniqueness(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines
		ADD CONSTRAINT pipelines_name_team_id UNIQUE (name, team_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipelines
		DROP CONSTRAINT constraint_pipelines_name_unique;
	`)
	return err
}
