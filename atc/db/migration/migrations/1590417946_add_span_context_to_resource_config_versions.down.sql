BEGIN;
  ALTER TABLE resource_config_versions DROP COLUMN span_context;
COMMIT;
