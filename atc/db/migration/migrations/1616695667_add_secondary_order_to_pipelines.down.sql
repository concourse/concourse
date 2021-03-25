BEGIN;
  ALTER TABLE pipelines DROP COLUMN secondary_ordering;
COMMIT;

