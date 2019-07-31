BEGIN;
  ALTER TABLE workers ADD COLUMN active_tasks integer DEFAULT 0 CHECK (active_tasks >= 0);
COMMIT;
