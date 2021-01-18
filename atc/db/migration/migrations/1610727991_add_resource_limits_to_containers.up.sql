BEGIN;
  ALTER TABLE containers ADD COLUMN meta_cpu_limit integer;
  ALTER TABLE containers ADD COLUMN meta_memory_limit integer;
COMMIT;
