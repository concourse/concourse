CREATE TABLE job_pauses (
  job_id integer PRIMARY KEY,
  paused boolean NOT NULL DEFAULT TRUE,
  paused_by text DEFAULT NULL,
  paused_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE job_pauses
  ADD CONSTRAINT job_pauses_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs (id) ON DELETE CASCADE;

INSERT INTO job_pauses
SELECT
  id,
  TRUE
FROM
  jobs
WHERE
  jobs.paused = TRUE;

ALTER TABLE jobs
  DROP COLUMN paused;
