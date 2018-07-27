BEGIN;

  ALTER TABLE workers DROP COLUMN reaper_addr;

COMMIT;
