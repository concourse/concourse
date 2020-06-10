BEGIN;
  ALTER TABLE resource_config_versions ADD COLUMN span_context jsonb;
COMMIT;
