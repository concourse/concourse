BEGIN;
ALTER TABLE builds
    ADD COLUMN created_by jsonb;
COMMIT;