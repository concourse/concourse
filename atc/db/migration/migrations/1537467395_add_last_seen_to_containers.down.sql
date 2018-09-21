BEGIN;
  ALTER TABLE containers DROP COLUMN last_seen;
COMMIT;
