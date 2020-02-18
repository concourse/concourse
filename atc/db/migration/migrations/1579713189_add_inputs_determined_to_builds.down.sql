BEGIN;
  ALTER TABLE builds
    DROP COLUMN inputs_ready;
COMMIT;
