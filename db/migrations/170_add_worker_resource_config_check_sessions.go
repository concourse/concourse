package migrations

import "github.com/concourse/atc/db/migration"

func AddWorkerResourceConfigCheckSessions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE worker_resource_config_check_sessions (
      id serial PRIMARY KEY,
      worker_base_resource_type_id integer REFERENCES worker_base_resource_types (id) ON DELETE CASCADE,
			resource_config_check_session_id integer REFERENCES resource_config_check_sessions (id) ON DELETE CASCADE
    )
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE resource_config_check_sessions
    DROP COLUMN worker_base_resource_type_id
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE containers
    DROP COLUMN resource_config_check_session_id
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE containers
    ADD COLUMN worker_resource_config_check_session_id integer REFERENCES worker_resource_config_check_sessions (id) ON DELETE SET NULL
  `)
	if err != nil {
		return err
	}

	return nil
}
