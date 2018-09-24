BEGIN;
  ALTER TABLE pipelines
  ADD COLUMN archived boolean NOT NULL DEFAULT false;
COMMIT;
