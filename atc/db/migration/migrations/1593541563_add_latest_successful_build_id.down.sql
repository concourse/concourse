BEGIN;
    ALTER TABLE jobs DROP COLUMN latest_successful_build_id;
COMMIT;
