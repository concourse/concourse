BEGIN;
    ALTER TABLE pipelines ADD COLUMN parent_job_id integer, ADD COLUMN parent_build_id integer;
COMMIT;
