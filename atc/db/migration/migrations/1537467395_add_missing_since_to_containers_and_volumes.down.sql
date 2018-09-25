BEGIN;
  ALTER TABLE containers DROP COLUMN missing_since;
  ALTER TABLE volumes DROP COLUMN missing_since;
COMMIT;
