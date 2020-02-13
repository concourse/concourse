BEGIN;

  CREATE UNIQUE INDEX build_names_uniq_idx ON builds (job_id, name);

  DROP INDEX builds_job_id;
  DROP INDEX builds_name;

COMMIT;
