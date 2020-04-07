BEGIN;
  ALTER TABLE pipelines
    DROP COLUMN "last_updated";
COMMIT;
