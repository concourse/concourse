BEGIN;
  CREATE UNIQUE INDEX worker_resource_caches_uniq
  ON worker_resource_caches (resource_cache_id, worker_base_resource_type_id);

  CREATE UNIQUE INDEX worker_task_caches_uniq
  ON worker_task_caches (job_id, step_name, worker_name, path);

  CREATE UNIQUE INDEX worker_resource_config_check_sessions_uniq
  ON worker_resource_config_check_sessions (resource_config_check_session_id, worker_base_resource_type_id, team_id);
COMMIT;
