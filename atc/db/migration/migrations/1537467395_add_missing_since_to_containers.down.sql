BEGIN;
  ALTER TABLE containers DROP COLUMN missing_since;
COMMIT;
