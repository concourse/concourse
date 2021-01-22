BEGIN;
ALTER TABLE builds
    ADD COLUMN created_by text not null default ''::text;
COMMIT;