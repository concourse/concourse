BEGIN;
  TRUNCATE TABLE next_build_inputs;

  ALTER TABLE next_build_inputs
    ADD COLUMN "version_md5" text,
    DROP COLUMN "id",
    DROP COLUMN "resource_config_version_id";
COMMIT;
