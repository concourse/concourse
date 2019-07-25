BEGIN;
  ALTER TABLE next_build_inputs
    DROP COLUMN resolve_error,
    ALTER COLUMN resource_config_version_id SET NOT NULL,
    ALTER COLUMN resource_id SET NOT NULL,
    ALTER COLUMN first_occurrence SET NOT NULL;
COMMIT;
