BEGIN;
  ALTER TABLE next_build_inputs
    ADD COLUMN resolve_error text,
    ALTER COLUMN resource_config_version_id DROP NOT NULL,
    ALTER COLUMN resource_id DROP NOT NULL,
    ALTER COLUMN first_occurrence DROP NOT NULL;
COMMIT;
