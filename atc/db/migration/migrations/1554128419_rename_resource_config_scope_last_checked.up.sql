BEGIN;
  ALTER TABLE resource_config_scopes RENAME COLUMN last_checked TO last_check_start_time;
  ALTER TABLE resource_config_scopes RENAME COLUMN last_check_finished TO last_check_end_time;
COMMIT;
