BEGIN;
  ALTER TABLE builds ADD tracked_by text;
COMMIT;
