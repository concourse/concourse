BEGIN;
  ALTER TABLE pipelines
  DROP COLUMN archived;
COMMIT;
