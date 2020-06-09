BEGIN;
    ALTER TABLE pipelines DROP COLUMN parent_job_id integer;
COMMIT;
