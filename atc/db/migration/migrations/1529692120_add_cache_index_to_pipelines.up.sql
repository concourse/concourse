BEGIN;
  ALTER TABLE pipelines ADD COLUMN cache_index integer NOT NULL DEFAULT 1;
  ALTER TABLE versioned_resources DROP COLUMN modified_time;
  ALTER TABLE build_inputs DROP COLUMN modified_time;
  ALTER TABLE build_outputs DROP COLUMN modified_time;
COMMIT;
