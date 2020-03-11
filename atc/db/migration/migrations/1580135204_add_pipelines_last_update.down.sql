BEGIN;
  ALTER TABLE pipelines
    DROP COLUMN IF EXISTS "last_updated";
COMMIT;
