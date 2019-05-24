BEGIN;
  ALTER TABLE build_resource_config_version_inputs DROP COLUMN first_occurrence;
COMMIT;
