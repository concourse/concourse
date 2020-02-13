BEGIN;

  CREATE INDEX builds_name ON builds USING btree (name);
  CREATE INDEX builds_job_id ON builds USING btree (job_id);

  DROP INDEX build_names_uniq_idx;

COMMIT;
