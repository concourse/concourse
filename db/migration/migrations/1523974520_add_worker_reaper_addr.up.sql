BEGIN;

  ALTER TABLE workers ADD COLUMN reaper_addr text;

COMMIT;
