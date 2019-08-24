BEGIN;
  ALTER TABLE pipelines ADD COLUMN cache_index integer NOT NULL DEFAULT 1;
COMMIT;
