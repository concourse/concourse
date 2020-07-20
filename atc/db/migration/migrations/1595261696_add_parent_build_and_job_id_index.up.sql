BEGIN;
  CREATE INDEX parent_job_id ON pipelines USING btree (parent_job_id);
  CREATE INDEX parent_build_id ON pipelines USING btree (parent_build_id);
COMMIT;
