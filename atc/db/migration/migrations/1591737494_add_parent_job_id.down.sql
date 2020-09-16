BEGIN;
    ALTER TABLE pipelines DROP COLUMN parent_job_id, DROP COLUMN parent_build_id;
COMMIT;
