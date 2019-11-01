BEGIN;
  ALTER TABLE pipelines
    DROP COLUMN schedule_requested;
COMMIT;
