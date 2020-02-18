BEGIN;
  CREATE INDEX builds_ordered_by_job_id_idx ON builds (job_id, id DESC);

  CREATE INDEX resource_config_versions_check_order_idx ON resource_config_versions (resource_config_scope_id, check_order DESC);

  CREATE INDEX next_build_id_idx ON builds (job_id) where status = 'pending' or status = 'started';
COMMIT;
