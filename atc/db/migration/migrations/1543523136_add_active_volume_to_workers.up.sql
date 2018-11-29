BEGIN;
  ALTER TABLE workers ADD COLUMN active_volumes integer DEFAULT 0;
COMMIT;
