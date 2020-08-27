BEGIN;
  ALTER TABLE resource_config_versions ALTER COLUMN id TYPE int;
COMMIT;
