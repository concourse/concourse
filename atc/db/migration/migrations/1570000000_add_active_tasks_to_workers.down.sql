BEGIN;
  ALTER TABLE workers DROP COLUMN active_tasks;
COMMIT;
