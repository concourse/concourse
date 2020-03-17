BEGIN;
  ALTER TABLE containers DROP COLUMN hijacked;
  ALTER TABLE containers DROP COLUMN discontinued;

  ALTER TABLE containers ADD COLUMN last_hijack timestamp with time zone;
COMMIT;
