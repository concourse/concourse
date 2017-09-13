package migrations

import "github.com/concourse/atc/db/migration"

func AddTeamIdToWorkerResourceConfigCheckSessions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	  ALTER TABLE worker_resource_config_check_sessions
		ADD COLUMN team_id integer REFERENCES teams (id) ON DELETE CASCADE
	`)
	if err != nil {
		return err
	}

	return nil
}
