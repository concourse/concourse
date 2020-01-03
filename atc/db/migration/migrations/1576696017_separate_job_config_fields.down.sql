BEGIN;
  ALTER TABLE jobs
    DROP COLUMN public,
    DROP COLUMN max_in_flight;
COMMIT;
