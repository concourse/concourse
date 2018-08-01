BEGIN;
  DROP INDEX worker_resource_caches_uniq;
  DROP INDEX worker_task_caches_uniq;
  DROP INDEX worker_resource_config_check_sessions_uniq;
COMMIT;
