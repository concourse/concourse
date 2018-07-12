BEGIN;
  ALTER TABLE jobs
  ADD COLUMN latest_completed_build_id integer REFERENCES builds (id) ON DELETE SET NULL,
  ADD COLUMN next_build_id integer REFERENCES builds (id) ON DELETE SET NULL,
  ADD COLUMN transition_build_id integer REFERENCES builds (id) ON DELETE SET NULL;

  CREATE INDEX jobs_latest_completed_build_id ON jobs (latest_completed_build_id);
  CREATE INDEX jobs_next_build_id ON jobs (next_build_id);
  CREATE INDEX jobs_transition_build_id ON jobs (transition_build_id);

  UPDATE jobs j SET latest_completed_build_id = v.id FROM latest_completed_builds_per_job v WHERE v.job_id = j.id;
  UPDATE jobs j SET next_build_id = v.id FROM next_builds_per_job v WHERE v.job_id = j.id;
  UPDATE jobs j SET transition_build_id = v.id FROM transition_builds_per_job v WHERE v.job_id = j.id;

  -- these are to be done in a later release
  -- DROP MATERIALIZED VIEW transition_builds_per_job;
  -- DROP MATERIALIZED VIEW next_builds_per_job;
  -- DROP MATERIALIZED VIEW latest_completed_builds_per_job;
COMMIT;
