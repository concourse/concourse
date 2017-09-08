package migrations

import "github.com/concourse/atc/db/migration"

func AddIndexesForResourceConfigCachesAndBuilds(tx migration.LimitedTx) error {

	_, err := tx.Exec(`CREATE INDEX resource_config_id ON public.resources(resource_config_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_cache_id ON public.build_image_resource_caches(resource_cache_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX builds_pipeline_id on public.builds(pipeline_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_cache_uses_container_id ON public.resource_cache_uses(container_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_types_resource_config_id ON public.resource_types(resource_config_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX worker_resource_config_check_sessions_worker_base_resource_type_id_idx ON public.worker_resource_config_check_sessions(worker_base_resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX worker_resource_config_check_sessions_resource_config_check_session_id_idx ON public.worker_resource_config_check_sessions(resource_config_check_session_id)`)
	if err != nil {
		return err
	}

	return nil
}
