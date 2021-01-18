BEGIN;
  ALTER TABLE containers DROP COLUMN meta_cpu_limit;
  ALTER TABLE containers DROP COLUMN meta_memory_limit;
COMMIT;
