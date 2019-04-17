BEGIN;
  ALTER TABLE pipelines DROP COLUMN cache_index;
COMMIT;
