BEGIN;
  DROP MATERIALIZED VIEW transition_builds_per_job;
  DROP MATERIALIZED VIEW next_builds_per_job;
  DROP MATERIALIZED VIEW latest_completed_builds_per_job;

  ALTER TABLE jobs
  ADD COLUMN latest_completed_build_id integer REFERENCES builds (id) ON DELETE SET NULL,
  ADD COLUMN next_build_id integer REFERENCES builds (id) ON DELETE SET NULL,
  ADD COLUMN transition_build_id integer REFERENCES builds (id) ON DELETE SET NULL;

  CREATE INDEX jobs_latest_completed_build_id ON jobs (latest_completed_build_id);

  CREATE INDEX jobs_next_build_id ON jobs (next_build_id);

  CREATE INDEX jobs_transition_build_id ON jobs (transition_build_id);
COMMIT;
