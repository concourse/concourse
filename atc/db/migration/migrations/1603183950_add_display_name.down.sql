BEGIN;
    ALTER TABLE resources DROP COLUMN display_name;
    ALTER TABLE jobs DROP COLUMN display_name;
COMMIT;
