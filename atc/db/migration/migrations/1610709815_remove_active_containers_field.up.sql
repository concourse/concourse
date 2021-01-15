BEGIN;
  ALTER TABLE workers DROP COLUMN active_containers;
COMMIT;
