BEGIN;
  ALTER TABLE workers ADD COLUMN active_tasks integer DEFAULT 0;
COMMIT;
