BEGIN;

  ALTER TABLE workers ALTER COLUMN reaper_addr SET DEFAULT ''::text;

COMMIT;
