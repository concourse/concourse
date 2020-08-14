BEGIN;
    ALTER TABLE pipelines ADD COLUMN display jsonb;
COMMIT;