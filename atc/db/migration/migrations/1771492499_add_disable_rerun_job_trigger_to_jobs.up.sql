ALTER TABLE jobs
  ADD COLUMN disable_rerun_job_trigger boolean NOT NULL DEFAULT FALSE;
