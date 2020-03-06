BEGIN;
  TRUNCATE TABLE resource_config_check_sessions;

  ALTER TABLE containers ADD COLUMN check_id bigint REFERENCES checks(id) ON DELETE NULL;

  ALTER TABLE containers DROP COLUMN resource_config_check_session_id;

COMMIT;
