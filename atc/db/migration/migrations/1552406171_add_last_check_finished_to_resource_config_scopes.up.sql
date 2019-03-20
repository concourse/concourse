BEGIN;

  ALTER TABLE resource_config_scopes
    ADD COLUMN last_check_finished timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00';

COMMIT;
