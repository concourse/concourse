BEGIN;
  ALTER TABLE workers DROP COLUMN active_volumes;
COMMIT;
