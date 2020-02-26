BEGIN;
  DROP INDEX builds_ordered_by_job_id_idx;

  DROP INDEX resource_config_versions_check_order_idx;

  DROP INDEX next_build_id_idx;
COMMIT;
