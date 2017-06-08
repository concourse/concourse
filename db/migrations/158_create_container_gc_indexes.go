package migrations

import "github.com/concourse/atc/db/migration"

func CreateContainerGCIndexes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE INDEX containers_image_check_container_id ON containers (image_check_container_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_image_get_container_id ON containers (image_get_container_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_resource_config_check_session_id ON containers (resource_config_check_session_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_config_check_sessions_resource_config_id ON resource_config_check_sessions (resource_config_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_config_check_sessions_worker_base_resource_type_id ON resource_config_check_sessions (worker_base_resource_type_id)`)
	if err != nil {
		return err
	}

	return nil
}
