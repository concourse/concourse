BEGIN;
  ALTER TABLE resource_configs DROP COLUMN last_referenced;
COMMIT;
