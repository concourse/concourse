BEGIN;
  ALTER TABLE containers
    ALTER COLUMN missing_since TYPE timestamp without time zone;

  ALTER TABLE volumes
    ALTER COLUMN missing_since TYPE timestamp without time zone;

  ALTER TABLE workers
    ALTER COLUMN start_time TYPE integer USING extract(epoch from start_time)::integer;
COMMIT;
