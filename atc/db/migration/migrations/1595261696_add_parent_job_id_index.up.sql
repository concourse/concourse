BEGIN;
  CREATE INDEX parent_job_id ON pipelines USING btree (parent_job_id);
COMMIT;
