BEGIN;
    ALTER TABLE jobs DROP COLUMN last_successful_build_id integer;
COMMIT;
