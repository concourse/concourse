package migrations

import "github.com/concourse/atc/db/migration"

func AddIndexesToEvenMoreForeignKeys(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE INDEX builds_pipeline_id ON builds (pipeline_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX resource_types_resource_config_id ON resource_types (resource_config_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX resource_cache_uses_container_id ON resource_cache_uses (container_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX worker_resource_config_check_sessions_resource_config_check_session_id ON worker_resource_config_check_sessions (resource_config_check_session_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX worker_resource_config_check_sessions_worker_base_resource_type_id ON worker_resource_config_check_sessions (worker_base_resource_type_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX resources_resource_config_id ON resources (resource_config_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX build_image_resource_caches_resource_cache_id ON build_image_resource_caches (resource_cache_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX volumes_worker_task_cache_id ON volumes (worker_task_cache_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX build_image_resource_caches_build_id ON build_image_resource_caches (build_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX containers_worker_resource_config_check_session_id ON containers (worker_resource_config_check_session_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX pipes_team_id ON pipes (team_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX worker_task_caches_worker_name ON worker_task_caches (worker_name)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX worker_task_caches_job_id ON worker_task_caches (job_id)
  `)
	if err != nil {
		return err
	}

	return nil
}
