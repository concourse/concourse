package migrations

import "github.com/concourse/atc/db/migration"

func AddResourceConfigCheckSessions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE resource_config_check_sessions (
			id serial PRIMARY KEY,
			resource_config_id integer REFERENCES resource_configs (id) ON DELETE CASCADE,
			worker_base_resource_type_id integer REFERENCES worker_base_resource_types (id) ON DELETE CASCADE,
			expires_at timestamp with time zone
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN resource_config_check_session_id integer REFERENCES resource_config_check_sessions (id) ON DELETE SET NULL,
		DROP COLUMN resource_config_id,
		DROP COLUMN worker_base_resource_type_id
	`)
	if err != nil {
		return err
	}

	return nil
}
