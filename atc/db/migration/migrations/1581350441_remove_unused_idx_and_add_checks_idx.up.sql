BEGIN;

  DROP INDEX builds_job_id_succeeded_idx;

  CREATE INDEX started_checks_idx ON checks (id) WHERE status = 'started';

COMMIT;
