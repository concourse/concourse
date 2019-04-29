BEGIN;

  ALTER TABLE builds
    DROP COLUMN create_time;

COMMIT;
