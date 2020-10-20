BEGIN;
    ALTER TABLE resources ADD COLUMN display_name text;
    ALTER TABLE jobs ADD COLUMN display_name text;
COMMIT;
