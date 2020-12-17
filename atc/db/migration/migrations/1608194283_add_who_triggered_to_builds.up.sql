BEGIN;
ALTER TABLE builds
    ADD COLUMN who_triggered text;
COMMIT;