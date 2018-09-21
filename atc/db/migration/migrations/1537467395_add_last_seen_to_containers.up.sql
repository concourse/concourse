BEGIN;
  ALTER TABLE containers ADD COLUMN last_seen timestamp without time zone;
COMMIT;
