BEGIN;
  CREATE INDEX builds_job_id_succeeded_idx ON builds (job_id, id DESC) where status = 'succeeded';
COMMIT;
