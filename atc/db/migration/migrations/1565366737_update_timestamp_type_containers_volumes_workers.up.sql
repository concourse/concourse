BEGIN;
  ALTER TABLE containers
    ALTER COLUMN missing_since TYPE timestamp with time zone;

  ALTER TABLE volumes
    ALTER COLUMN missing_since TYPE timestamp with time zone;

  ALTER TABLE workers
    ALTER COLUMN start_time TYPE timestamp with time zone USING to_timestamp(start_time);
COMMIT;
