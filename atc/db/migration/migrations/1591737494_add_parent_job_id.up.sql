BEGIN;
    ALTER TABLE pipelines ADD COLUMN parent_job_id integer;
COMMIT;
