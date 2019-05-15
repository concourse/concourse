BEGIN;

  ALTER TABLE resource_config_scopes
    DROP COLUMN last_check_finished;

COMMIT;
