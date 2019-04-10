BEGIN;
  ALTER TABLE resource_config_scopes RENAME COLUMN last_check_start_time TO last_checked;
  ALTER TABLE resource_config_scopes RENAME COLUMN last_check_end_time TO last_check_finished;
COMMIT;
