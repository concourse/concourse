BEGIN;
  ALTER TABLE resources
    DROP COLUMN paused;
COMMIT;
