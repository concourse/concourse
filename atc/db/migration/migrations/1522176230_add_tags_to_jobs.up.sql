BEGIN;

  ALTER TABLE jobs ADD COLUMN tags text[];

COMMIT;
