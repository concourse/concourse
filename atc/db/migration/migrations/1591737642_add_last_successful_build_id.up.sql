BEGIN;
    ALTER TABLE jobs ADD COLUMN last_successful_build_id integer;
COMMIT;
