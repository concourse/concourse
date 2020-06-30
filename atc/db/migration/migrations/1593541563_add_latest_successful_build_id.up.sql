BEGIN;
    ALTER TABLE jobs ADD COLUMN latest_successful_build_id integer;
COMMIT;
