BEGIN;
  ALTER TABLE jobs
    DROP COLUMN public,
    DROP COLUMN max_in_flight,
    DROP COLUMN disable_manual_trigger;
COMMIT;
