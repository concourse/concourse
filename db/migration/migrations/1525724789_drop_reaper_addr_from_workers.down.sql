BEGIN;

  ALTER TABLE workers ADD COLUMN reaper_addr DEFAULT ''::text;

COMMIT;
