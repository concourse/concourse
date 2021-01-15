BEGIN;
  ALTER TABLE workers ADD COLUMN active_containers integer DEFAULT 0;
COMMIT;
