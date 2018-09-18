BEGIN;

  ALTER TABLE workers ADD reaper_addr text DEFAULT '';

COMMIT;
