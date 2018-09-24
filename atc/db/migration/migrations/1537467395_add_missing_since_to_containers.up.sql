BEGIN;
  ALTER TABLE containers ADD COLUMN missing_since timestamp without time zone;
COMMIT;
